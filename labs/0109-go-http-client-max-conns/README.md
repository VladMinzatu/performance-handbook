# Go HTTP client MaxConnsPerHost: a connection limit is a queue that fills when the downstream slows

Uses the shared lab infrastructure in [tools/](../tools/README.md). The
`analysis` container's `ss` and `bpftrace` show that the network and the
backend look healthy. The client's goroutine dump shows where the time
actually goes.

## Background

Go's `http.Transport` has two per-host connection settings that are
easy to confuse:

- `MaxIdleConnsPerHost` (default 2) - how many connections to keep open
  *between* calls.
- `MaxConnsPerHost` (default **0, no limit**) - how many connections may
  exist to a host at once, whether dialing, in use or idle.

With the default, a call that finds no idle connection always dials a
new one, so the number of connections follows the number of calls in
flight. Most services never change this. Widely used projects that
expose the setting, such as Grafana's data source proxy, Caddy's
reverse proxy and the Thanos object storage client, also default it to
0 and leave the value to operators.

Setting it changes what the client does when every connection is busy.
A call that finds no idle connection and the host at its limit doesn't
dial. It waits until a connection is handed back, and calls take
returned connections in arrival order. So a connection limit is a
queue in front of the downstream, inside the client process.

That queue has a fixed capacity. By Little's law, calls in flight =
rate × latency. With at most `N` connections, each carrying one call at
a time over HTTP/1.1, the client can complete at most `N` / latency
calls per second. With a limit of 10 and a downstream that answers in
25ms, that's 400 calls/s. If the downstream slows to 50ms, the same
limit allows only 200 calls/s. Capacity drops exactly when the
downstream gets slower.

Below capacity, the limit is mostly invisible. Above it, calls arrive
faster than connections free up. The queue grows for as long as that
lasts, and so does latency. Nothing bounds the wait except the call's
own context or `Client.Timeout`. The `Transport` has no separate
"wait for a connection" timeout, so a call can spend its whole timeout
budget waiting and then fail without ever reaching the downstream.
Each waiting call is a goroutine holding its request, so memory grows
with the queue.

This is hard to spot because everything outside the client looks
healthy:

- **The downstream** sees a steady number of connections, exactly the
  limit, and a request rate capped at `N` / latency. Its own latency
  only reflects the actual slowdown, not the time calls spend queued.
- **The network** shows no churn, retransmits or errors: the same `N`
  connections stay open.
- **The client's latency metrics**, if they time the whole call, include
  the queueing. So the latency for "calls to the downstream" looks much
  worse than the downstream says it is.

The gap between the client's latency and the downstream's own latency
is the time spent waiting for a connection. That time sits in user
space: goroutines blocked inside `Transport.getConn`, visible in a
goroutine dump.

### When a limit is worth setting

A limit makes sense when connections, not requests, are the scarce
resource, or when the client must not flood a struggling downstream:

- **The downstream has a hard connection limit or a high cost per
  connection.** Examples are thread-per-connection servers, legacy
  services, proxies with `maxconn` settings, and APIs that throttle or
  reject clients opening too many connections.
- **Protecting a slow downstream.** Without a limit, a slowdown makes
  the client open more connections: calls in flight = rate × latency,
  so a downstream that goes from 25ms to 1s at 300 calls/s faces about
  300 connections instead of about 8. The limit caps the connections
  and load the downstream sees, and the client queues instead.
- **Protecting the client host.** A limit bounds file descriptors and
  ephemeral ports during bursts or connection storms.

### What can go wrong once it's set

- **The capacity collapses when the downstream slows.** The limit is
  usually sized from normal latency. Under a slowdown, `N` / latency
  falls below the request rate, and the queue grows without bound.
- **It's a queue with no timeout of its own.** Calls wait until their
  context or `Client.Timeout` expires. Pair the limit with a deadline,
  and ideally a fail-fast limit on in-flight calls, so overload turns
  into fast errors rather than a backlog of goroutines.
- **It's per `Transport`, per host.** A downstream with many client
  instances sees each instance's limit added up. A limit that protects
  the downstream at 5 replicas doesn't at 50.
- **HTTP/2 changes its meaning.** It counts connections, and each HTTP/2
  connection carries many calls. Go has also had bugs in this code
  path: hangs and crashes with HTTP/2 and a low limit
  ([golang/go#34941](https://github.com/golang/go/issues/34941)), and a
  panic from the connection count going negative
  ([golang/go#38172](https://github.com/golang/go/issues/38172)).

## Hypotheses

The client sends 300 calls/s over HTTP/1.1 with an idle limit large
enough that connections are always reused. Under normal load the
backend takes 10-40ms, so about 8 calls are in flight. The slowdown
doubles that to 20-80ms, about 15 calls in flight.

**Prediction 1: a limit below the calls in flight turns a slowdown into
an unbounded queue.** With `MAX_CONNS_PER_HOST=10` and normal latency,
the client should behave about the same as with no limit: about 300
calls/s at about 25ms. After the backend slows down, it should complete
only about 200 calls/s (10 connections / 50ms). Client latency should
keep climbing, the number of goroutines should grow by about 100/s,
and after a while calls should fail with the client's 5s timeout. The
same slowdown with no limit should just double latency to about 50ms,
with about 15 connections and no errors.

**Prediction 2: from the outside, the network and the backend look
healthy, and the wait shows up only in the client's goroutines.** With
the limit hit, `ss` should show exactly 10 established connections, and
`bpftrace` should count no new connects. The backend should log about
200 requests/s, 10 open connections and its own latency of about 50ms.
A goroutine dump of the client should show hundreds of goroutines
blocked in `net/http.(*Transport).getConn`, about the length of the
queue. The client's latency minus the backend's latency is the time
spent waiting there.

**Prediction 3: without a limit, a slow downstream gets a connection
for every call in flight.** With the backend slowed to about 1s, the
client with no limit should hold about 300 connections to it (300
calls/s × 1s) and keep calls flowing at 1s latency. With
`MAX_CONNS_PER_HOST=10`, the backend should see 10 connections and
about 10 requests/s, and nearly every call should time out in the
client. The limit protects the downstream's connection count at the
cost of failing calls in the client. That's the trade-off a limit sets.
