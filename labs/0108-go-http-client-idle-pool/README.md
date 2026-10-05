# Go HTTP client idle pool: connection churn from MaxIdleConnsPerHost

Uses the shared lab infrastructure in [tools/](../tools/README.md) - the
`analysis` container's `ig` and `ss` are how this lab sees connection
churn from the outside, without any help from the client's own code.

## Background

Almost every Go backend calls other services over HTTP through
`net/http`'s `Transport`, usually one `Transport` (often just
`http.DefaultTransport`) shared by every request. The `Transport` keeps a
pool of open connections per host so calls can reuse them instead of
dialing a new one each time. How many connections it keeps is set by
`MaxIdleConnsPerHost`, which defaults to **2**.

The idle limit doesn't cap how many connections the client opens:
`MaxConnsPerHost` defaults to unlimited, so every concurrent call gets a
connection, dialing a new one if none is idle. The limit applies when a
call finishes and hands its connection back. If the pool already holds
2 idle connections to that host, the returned connection is closed
instead. A service making ~15 concurrent calls to the same host,
steadily, ends up with 15 connections in use, but whenever a few calls
finish close together, all but 2 of their connections are closed - and
the next calls to start find the pool empty and dial again. The client
opens and closes connections constantly, even at a steady rate.
(`MaxIdleConns`, default 100, is a separate limit on idle connections
across all hosts, and `IdleConnTimeout`, default 90s, closes connections
that stay idle too long - neither matters for a single busy host.)

Every one of those reconnects has a cost:

- **Latency.** A new connection costs one network round trip (the TCP
  handshake) before the request can be sent - plus one or two more for
  TLS, depending on the version. On
  a local network that's well under a millisecond; between zones or
  regions it's 1-50ms, added to a large fraction of calls.
- **`TIME_WAIT` sockets.** The side that closes a TCP connection first
  keeps it in `TIME_WAIT` for 60 seconds on Linux. Here, that's the
  client. At a steady churn rate, the number of `TIME_WAIT` sockets
  settles at about connect rate × 60s.
- **Ephemeral ports.** Each `TIME_WAIT` socket holds on to a local port
  for its destination. Connections to one destination (a single IP and
  port, e.g. a load balancer or a Kubernetes service) can use only
  `ip_local_port_range` - about 28,000 ports by default. That runs out at
  roughly 28,000 / 60s ≈ 470 new connections per second, after which
  dialing fails with `connect: cannot assign requested address`.
- **CPU**, on both ends, for the handshakes and the socket setup and
  teardown - much more with TLS.

Raising `MaxIdleConnsPerHost` to at least the peak number of concurrent
calls to a host fixes all of it: once warmed up, the pool has enough idle
connections for every call, and the client stops dialing. In production,
the problem is easy to spot from the outside: a connect rate to a
downstream that's a large fraction of the request rate, and a `TIME_WAIT`
count in the thousands.

## Hypotheses

**Prediction 1 - with the default idle limit, the client churns
connections at a steady request rate.** At 300 req/s to a 50ms backend
(about 15 concurrent calls), `ig trace_tcp` on the client should show
connects to the backend at a large fraction of the request rate, each
paired with a close, and `httptrace` should report a similar share of
calls dialing a new connection. `ss -s` in the client's network namespace
should show `TIME_WAIT` sockets settling at about connect rate × 60.

**Prediction 2 - churn stops once the idle limit covers peak
concurrency, not average concurrency.** Sweeping `MAX_IDLE_CONNS_PER_HOST`
upwards from 2 should lower the connect rate gradually, not in one step,
and reach ~0 only once the limit is at or above the peak number of
concurrent calls - higher than the ~15 average, because requests don't
finish evenly spaced. `TIME_WAIT` sockets should drain to 0 within 60
seconds of the churn stopping.

**Prediction 3 - the latency cost of churn is one round trip per new
connection, so it depends on the network, not on the client.** On the
local container network, the difference in latency between the default
and a large enough idle limit should be negligible. Adding 2ms of delay
to the backend's traffic should make it obvious: calls that dial pay
about one extra RTT, so the median shifts by around 2ms × the share of
calls dialing, and the p99 by a full RTT.

**Prediction 4 (stretch) - enough churn to one destination exhausts
ephemeral ports.** Narrowing the client's `ip_local_port_range` to a few
thousand ports (to reach the limit at lab-scale rates) should make dial
errors appear once `TIME_WAIT` sockets fill the range - at about
range size / 60 connects per second. The same load with a large enough
idle limit should make no dial errors at all.

## Setup

Build and start the system under test, and make sure the analysis
container is running:
```sh
docker compose -f compose.yml up -d --build
docker compose -f ../tools/analysis/compose.yml up -d --build
```
- `lab-go-idlepool-backend` - the downstream. Handles each request in
  `SERVICE_MS` (default 50ms) and prints its request rate, latency, and
  number of new connections accepted once per second.
- `lab-go-idlepool-client` - the service under study. `GET /call` makes
  one call to the backend through a single shared `http.Client` whose
  `Transport` has `MaxIdleConnsPerHost` set from `MAX_IDLE_CONNS_PER_HOST`
  (default 2, Go's default) and every other setting at Go's defaults.
  Prints its rate, latency, dial errors, and the share of calls that
  dialed a new connection (from `httptrace`) once per second.
- `lab-go-idlepool-loadgen` - an open-loop load generator: requests are
  sent at a fixed rate whether or not earlier ones have completed, the way
  independent callers behave.

All three join `labnet`, so the analysis container can reach them by
name.

Generate load (rate in requests/second):
```sh
docker exec lab-go-idlepool-loadgen loadgen -url http://client:8080/call -rate 300 -duration 60s
```

Watch both sides:
```sh
docker logs -f lab-go-idlepool-client
docker logs -f lab-go-idlepool-backend
```

Connects and closes made by the client:
```sh
docker exec lab-analysis ig run trace_tcp:latest --containername lab-go-idlepool-client
```

The client's socket summary (including `TIME_WAIT`), from inside its
network namespace:
```sh
CLIENT_PID=$(docker inspect -f '{{.State.Pid}}' lab-go-idlepool-client)
docker exec lab-analysis nsenter --net=/proc/$CLIENT_PID/ns/net -- ss -s
```

Network delay on the backend's traffic (Prediction 3), and removing it:
```sh
BACKEND_PID=$(docker inspect -f '{{.State.Pid}}' lab-go-idlepool-backend)
docker exec lab-analysis nsenter --net=/proc/$BACKEND_PID/ns/net -- tc qdisc add dev eth0 root netem delay 2ms
docker exec lab-analysis nsenter --net=/proc/$BACKEND_PID/ns/net -- tc qdisc del dev eth0 root
```

The client's local port range (Prediction 4):
```sh
docker exec lab-analysis nsenter --net=/proc/$CLIENT_PID/ns/net -- sysctl -w net.ipv4.ip_local_port_range="40000 42999"
```

To try a different idle limit, edit `MAX_IDLE_CONNS_PER_HOST` in
`compose.yml` and recreate the client:
```sh
docker compose -f compose.yml up -d --build client
```

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down
```
