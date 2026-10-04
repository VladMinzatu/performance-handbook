# HTTP client connection-pool exhaustion: latency that isn't the downstream's

Uses the shared lab infrastructure in [tools/](../tools/README.md) - the
`analysis` container is how this lab sees inside the client: its sockets
(`ss`), its connection activity (`ig`), and per-socket request/response
timing (`bpftrace`).

## Background

The symptom this lab reproduces is a common one: a service's latency to
its callers climbs and keeps climbing, while the downstream it depends on
reports perfectly steady latency on its own dashboards. Both sides are
telling the truth. The time is being spent somewhere neither one measures
by default: in the client, waiting for a connection to the downstream to
become free.

Go's `net/http.Transport` keeps a pool of connections per host, and
`MaxConnsPerHost` caps how many can exist at once - dialing, in use, or
idle. Once that many are busy, a new request doesn't get a new connection
and doesn't fail either: it blocks inside `Transport.getConn` until one of
the busy connections finishes its request and is handed over. Nothing about
that wait is visible from the downstream (it never sees the request until
the wait is over), on the network (no packets are sent), or in CPU usage
(blocked goroutines cost nothing). The pool cap that protects the
downstream from too many connections also sets a hard ceiling on
throughput to it.

That ceiling follows directly from Little's law (L = λW): with `N`
connections each busy for the downstream's service time `S` per request,
the client can complete at most `N / S` requests per second through the
pool. Below that rate, the pool is a non-issue and the client's latency is
roughly `S`. Above it, requests arrive faster than connections free up, and
the difference queues in front of the pool. With traffic that doesn't slow
down when responses do (real users, open-loop load), that queue - and the
latency of every request in it - grows for as long as the overload lasts.
This doesn't need the downstream to be overloaded: the same ceiling is hit
when a healthy downstream simply gets a bit slower, since raising `S`
lowers `N / S`.

`MaxConnsPerHost` defaults to 0 (unlimited), so this usually appears
because someone set it on purpose - to protect a downstream, or to stay
under a connection limit - and sized it for normal latency, not degraded
latency. The same pattern applies to any bounded client-side pool:
database connection pools, gRPC channel limits, semaphores around outbound
calls.

## Hypotheses

**Prediction 1 - throughput through the pool is capped at
`MAX_CONNS / SERVICE_MS`, regardless of offered load.** With
`MAX_CONNS=20` and `SERVICE_MS=50`, the ceiling is 400 req/s (in
practice a little lower, since a round trip takes a bit more than the
50ms of service time). Below it, edge's latency should match inventory's
service time. Above it, edge's completed rate should stay pinned at the
ceiling, and its latency should grow steadily over time instead of
settling, while inventory's self-reported latency stays at ~50ms and its
request rate stays at the same ceiling. Raising inventory's service time at runtime (with load held below
the original ceiling) should cause the same collapse, since a slower
downstream lowers the ceiling.

**Prediction 2 - the socket table shows a saturated pool, not a
struggling network.** In edge's network namespace, `ss` should show
exactly `MAX_CONNS` established connections to inventory, never more, and
each one should have low RTT and empty send/receive queues - so the
connections are healthy and fully used. `ig trace_tcp` on edge should show
almost no new connects while the overload lasts: the pool isn't churning,
it's just full.

**Prediction 3 - the goroutine dump shows where the waiting happens.**
Edge's goroutine profile under overload should split into two groups:
about `MAX_CONNS` goroutines waiting on an in-flight response (in the
persistent connection's round trip), and everything else parked in
`net/http.(*Transport).getConn`, waiting for a connection. The size of
the second group should track the queue implied by Little's law: in-flight
requests (offered rate × observed latency) minus `MAX_CONNS`.

**Prediction 4 - per-socket timing on the client confirms the downstream
isn't the slow part, without changing any code.** A `bpftrace` histogram of
the time from a request being written to an inventory socket to its first
response bytes arriving, filtered to edge's cgroup, should stay at
~`SERVICE_MS` while edge's end-to-end latency grows by orders of
magnitude. The difference between the two is the pool wait. Edge's own
`httptrace` split (time to get a connection vs time to the first response
byte) is printed as a cross-check, but the point is that the OS-level view
gets the same answer without it.

**Prediction 5 (stretch) - raising `MAX_CONNS` moves the bottleneck, it
doesn't remove it.** Raising `MAX_CONNS` should raise the ceiling
proportionally, until inventory's own capacity (`WORKERS` concurrent
requests) becomes the limit. At that point, the queue should move
downstream: edge's goroutines stop piling up in `getConn` and pile up
waiting on responses instead, `ss` shows more connections, and inventory's
self-reported latency now grows too. The pool cap was hiding a capacity
limit, not causing one.

## Setup

Build and start the system under test, and make sure the analysis
container is running:
```sh
docker compose -f compose.yml up -d --build
docker compose -f ../tools/analysis/compose.yml up -d --build
```
- `lab-pool-inventory` - the downstream. Handles each request in
  `SERVICE_MS` (default 50ms), at most `WORKERS` (default 200) at a time.
  Prints its own request rate and latency once per second. Its service
  time can be changed at runtime:
  `docker exec lab-analysis curl -s 'http://inventory:8080/admin/service-time?ms=100'`.
- `lab-pool-edge` - the client service. `GET /checkout` makes one call to
  inventory through a shared `http.Client` with `MaxConnsPerHost=MAX_CONNS`
  (default 20; `MaxIdleConnsPerHost` set to the same value, so idle
  connections aren't closed between requests). No client timeout, so
  requests wait rather than fail. Prints its completed rate, end-to-end
  latency, and `httptrace` connection-wait vs time-to-first-byte once per
  second. `pprof` is served on `:6060`.
- `lab-pool-loadgen` - an open-loop load generator: it sends requests at
  a fixed rate whether or not earlier ones have completed, the way real
  traffic behaves.

All three join `labnet`, so the analysis container can reach them by
name.

Generate load (rate in requests/second):
```sh
docker exec lab-pool-loadgen loadgen -url http://edge:8080/checkout -rate 300 -duration 60s
```

Watch both sides:
```sh
docker logs -f lab-pool-edge
docker logs -f lab-pool-inventory
```

Edge's sockets to inventory:
```sh
EDGE_PID=$(docker inspect -f '{{.State.Pid}}' lab-pool-edge)
docker exec lab-analysis nsenter --net=/proc/$EDGE_PID/ns/net -- ss -tin state established '( dport = :8080 )'
```

Edge's goroutine dump:
```sh
docker exec lab-analysis curl -s 'http://edge:6060/debug/pprof/goroutine?debug=1'
```

TCP connect/accept/close events from edge:
```sh
docker exec lab-analysis ig run trace_tcp:latest --containername lab-pool-edge
```

Edge's cgroup, for `bpftrace` filters:
```sh
CID=$(docker inspect --format '{{.Id}}' lab-pool-edge)
# then, in a bpftrace filter: /cgroup == cgroupid("/host/sys/fs/cgroup/docker/$CID")/
```

To try other values of `MAX_CONNS`, `SERVICE_MS`, or `WORKERS`, edit the
`environment:` blocks in `compose.yml` and recreate:
```sh
docker compose -f compose.yml up -d --build
```

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down
```
