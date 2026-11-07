# Go HTTP client idle pool: MaxIdleConnsPerHost=2 turns steady load into connection churn

Uses the shared lab infrastructure in [tools/](../tools/README.md). The
`analysis` container's `bpftrace` and `ss` show the churn from the
outside, without any help from the client's own code.

## Background

Almost every Go backend calls other services over HTTP through one shared
`net/http` `Transport`, often just `http.DefaultTransport`. The
`Transport` keeps a pool of open connections per host so calls can reuse
them instead of dialing each time. How many idle connections it keeps per
host is set by `MaxIdleConnsPerHost`. Left at zero, which is the case in
`DefaultTransport` and any `Transport{}` that doesn't set it, it means
**2**.

The 2 is a holdover from HTTP/1.1's original spec (RFC 2616), which told
single-user clients such as browsers to keep at most 2 connections per
server. The current spec (RFC 7230) dropped that limit, but Go's default
stayed. It suits a browser, not a backend fanning out to one downstream.
Widely used client libraries replace it with their own value: the AWS
SDK for Go v2 uses 10, Kubernetes' client-go uses 25, and HashiCorp's
`go-cleanhttp` (used by Vault, Consul and Terraform) uses
`GOMAXPROCS + 1`. Projects such as Prometheus, Cortex and GitLab Pages
have raised it after running into churn
([golang/go#13801](https://golang.org/issue/13801)). Application code
that uses `http.DefaultClient`, `http.Get` or a bare `&http.Transport{}`
still gets 2, and that's very common.

The limit counts only *idle* connections, the ones in the pool with no
request on them. Nothing caps how many connections are open:
`MaxConnsPerHost` is unlimited by default. When a call needs a
connection, the `Transport` does two things at once:

1. It looks for an idle connection to the host. If there's none, it
   registers the call as waiting for the next one that's returned.
2. Without waiting, it starts dialing a new connection in the
   background.

The call takes whichever comes first. A connection that's returned goes
straight to a waiting call (Go's source calls this *late binding*, after
Chrome). The dial isn't cancelled when that happens: its context is cut
off from the request's on purpose, so that a later call can use the new
connection. When the dial finishes and nobody needs it, the connection
goes into the idle pool, or is closed if the pool is already full. Calls
wait for a connection instead of dialing only when `MaxConnsPerHost` is
set and the host has reached it.

So the idle limit applies only when a connection is handed back. If 2
connections to that host are already idle, the returned connection, or a
newly dialed one nobody took, is closed. Real traffic is uneven: the
number of calls in flight goes up and down from one millisecond to the
next. Every rise dials new connections, and every fall closes all but 2
of them. So the client opens and closes connections constantly, even at
a steady request rate. (`MaxIdleConns`, default 100, limits idle
connections across all hosts, and `IdleConnTimeout`, default 90s, closes
connections that stay idle too long. Neither matters for a single busy
host.)

This only happens with HTTP/1.1, where a connection carries one request
at a time. Over TLS to a server that speaks HTTP/2, which
`DefaultTransport` tries first, one connection carries many requests at
once, and the idle limit hardly matters. gRPC is HTTP/2 as well. Where it
hurts is plaintext `http://` between services inside a cluster, which is
HTTP/1.1. That includes calls from the app to a service-mesh sidecar on
localhost, where the churn just moves to loopback.

Requests still succeed and error rates stay at zero. The cost shows up
elsewhere:

- **Latency.** Each new connection adds a TCP handshake round trip before
  the request is sent, plus one or two more with TLS. That's well under a
  millisecond on a local network and 1-50ms between zones or regions.
- **CPU** on both ends for socket setup and teardown, and much more for
  TLS handshakes.
- **`TIME_WAIT` sockets.** The side that closes a TCP connection first
  keeps it in `TIME_WAIT` for 60s on Linux. Here that side is the client,
  so at a steady churn rate the client holds about connect rate × 60
  `TIME_WAIT` sockets.
- **Ephemeral ports.** Each of those sockets holds a local port for its
  destination. A single destination, such as a load balancer or a
  Kubernetes service IP, gets only `ip_local_port_range`, which is about
  28,000 ports by default. That runs out at about 470 new connections per
  second, after which dials fail with `connect: cannot assign requested
  address`.

The fix is to set `MaxIdleConnsPerHost` to at least the *peak* number of
concurrent calls to the host. Once the pool has warmed up, every call
finds an idle connection and the client stops dialing. From the outside
the problem is easy to spot: the client's connect rate to a downstream is
a large fraction of its request rate, and its `TIME_WAIT` count is in the
thousands.

## Hypotheses

**Prediction 1: with the default idle limit, a large share of calls dial
a new connection.** At 300 req/s to a backend that takes 10-40ms (about
7-8 calls in flight on average, more during bursts), the kernel's
`tcp_connect` count for the client's cgroup should be a large fraction of
the request rate: tens of connects per second or more, not about 0. The
client's `httptrace` dial share should agree. `TIME_WAIT` sockets in the
client's network namespace should settle at about connect rate × 60, in
the thousands.

**Prediction 2: the client is the side that closes.** Counting
`tcp_time_wait` by cgroup should show essentially every `TIME_WAIT` entry
in the client's cgroup and none in the backend's. The churn comes from
the client's pool setting, not from the server's keep-alive behavior.

**Prediction 3: an idle limit at or above peak concurrency stops the
churn.** With `MAX_IDLE_CONNS_PER_HOST=64` and the same load, the client
should dial a handful of connections while warming up and then none at
all. `ss` should show about 10-20 established connections to the
backend, and the `TIME_WAIT` count should drain to 0 within 60s.
