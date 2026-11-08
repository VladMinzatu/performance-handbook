## An idle pool sized for peak concurrency

Tests **Prediction 3**: with `MaxIdleConnsPerHost` at or above the peak
number of calls in flight, the client stops dialing. The load is the
same as before: 300 calls/s to a backend that takes 10-40ms, with
13-19 calls in flight at peak. The only change is
`MAX_IDLE_CONNS_PER_HOST=64`, comfortably above that peak. The
measurements are the same as for the default, plus one check that the
client keeps using the same connections over time.

Changing the setting recreates the client container, and a new
container gets a new network namespace. Any `TIME_WAIT` sockets from an
earlier run disappear with the old namespace, so they can't be watched
draining here. The check is instead whether `TIME_WAIT` stays at about
0 under load, long after the 60s it would take to refill.

Start the analysis container if it isn't running, then the lab with the
larger idle limit, from this lab's directory. If the backend is already
running, only the client is recreated:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
MAX_IDLE_CONNS_PER_HOST=64 docker compose -f compose.yml up -d --build client
```
Wait at least 60s before taking the measurements below.

One small source of dials and closes is expected. The pool hands out
the most recently used idle connection first. Connections opened during
the biggest bursts sit at the bottom and may not be used again for a
while. After 90s idle (`IdleConnTimeout`), the client closes them, and
the next burst that big dials again. That should be a few connections a
minute at most, compared with about 113 per second with the default.

### The client's own view

```sh
docker logs --tail 15 lab-go-idlepool-client
```
producing output:
```
t=   5s ok= 1492 err=  0 p50=26.5ms  p99=43.5ms  new=  0.6% dials=   15 peak=15
t=  10s ok= 1498 err=  0 p50=26.7ms  p99=42.8ms  new=  0.0% dials=    0 peak=13
t=  15s ok= 1502 err=  0 p50=26.7ms  p99=42.5ms  new=  0.0% dials=    0 peak=13
t=  20s ok= 1497 err=  0 p50=26.6ms  p99=42.4ms  new=  0.0% dials=    0 peak=13
t=  25s ok= 1504 err=  0 p50=27.1ms  p99=42ms    new=  0.0% dials=    0 peak=13
```
The first window should show the warmup dials, about as many as the
peak number of calls in flight. After that, `new` and `dials` should be
about 0.

That's what happens. The first window has 15 dials with a peak of 15
calls in flight: the pool fills to one connection per call in flight
during the first burst, then stops dialing. Only 0.6% of calls in that
window, about 9, ran on a connection dialed for them. The other 6
dials were overtaken by a returned connection, the same race seen with
the default limit. Here it costs nothing, because the pool has room
for the extra connection instead of closing it. From the second window
on, `new` and `dials` are 0.

Throughput and latency (p50 about 27ms, p99 about 43ms) are the same
as with the default limit. On this network, the churn cost nothing in
latency, so removing it doesn't improve latency either.

### The backend's view

```sh
docker logs --tail 5 lab-go-idlepool-backend
```
producing output:
```
t=  40s requests= 1498 accepted=    0 open=15
t=  45s requests= 1502 accepted=    0 open=15
t=  50s requests= 1503 accepted=    2 open=17
t=  55s requests= 1495 accepted=    0 open=17
t=  60s requests= 1503 accepted=    0 open=17
```
`accepted` should be about 0 per window, and `open` should hold steady
at about the client's highest `peak` so far.

`accepted` is 0 in every window except one, which has 2 new
connections, and at the same moment `open` goes from 15 to 17. That's
a burst with more calls in flight than ever before. With no idle
connection free, those calls dialed, and the 2 new connections then
stayed in the pool. It's too early in the run for `IdleConnTimeout`
(90s) to have closed anything, so these are growth, not replacements.

So the pool doesn't settle at the average concurrency (about 8) but at
the most calls ever in flight at once. Rare, larger bursts add a
connection now and then. A limit of 64 doesn't open 64 connections. It
only lets the pool keep every connection the load has needed so far.

### Connect rate, from the kernel

```sh
CLI=$(docker inspect --format '{{.Id}}' lab-go-idlepool-client)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e 'kprobe:tcp_connect /cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${CLI}\")/ { @connects = count(); }'"
```
producing output:
```
Attaching 1 probe...

@connects: 0
```
No connects in 10s at 300 calls/s, compared with 1127 with the default
limit. That's 0 connects per call, the healthy value.

### Which side closes first

```sh
CLI=$(docker inspect --format '{{.Id}}' lab-go-idlepool-client)
SRV=$(docker inspect --format '{{.Id}}' lab-go-idlepool-backend)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e '
tracepoint:sock:inet_sock_set_state /args.newstate == 4 && cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${CLI}\")/ { @closed_first[\"client\"] = count(); }
tracepoint:sock:inet_sock_set_state /args.newstate == 4 && cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${SRV}\")/ { @closed_first[\"backend\"] = count(); }'"
```
producing output:
```
Attaching 2 probes...
```
Neither side closed a connection in 10s, compared with 1098 closes by
the client with the default limit. Connections that aren't closed
never reach `TIME_WAIT`.

### Sockets by state

```sh
for c in lab-go-idlepool-client lab-go-idlepool-backend; do
  PID=$(docker inspect --format '{{.State.Pid}}' $c)
  for st in established time-wait; do
    printf '%s %s ' $c $st
    docker exec lab-analysis nsenter --net=/proc/$PID/ns/net ss -tanH state $st | wc -l
  done
done
```
producing output:
```
lab-go-idlepool-client established       19
lab-go-idlepool-client time-wait          0
lab-go-idlepool-backend established      19
lab-go-idlepool-backend time-wait         0
```
`TIME_WAIT` is 0 on the client, compared with 7264 with the default
limit, measured more than 60s into the run. With 0 closes, it can't
fill up again. The client has 19 established connections, slightly
more than the 17 the backend reported earlier. That's the high-water
mark rising with an occasional larger burst, as seen in the backend's
log. Both sides agree on the count: each connection is one socket at
each end.

That's about twice the 7-8 connections open with the default limit, but
they're the same connections all the time. The cost of the fix is a few
more idle sockets and file descriptors on each side.

### Same connections over time

Zero connects means no new connections. The other half of reuse is that
the existing connections keep being used rather than replaced. This
lists the client's local ports for its established connections twice,
30s apart, and compares the two lists. Each connection has its own
local port, so the same set of ports means the same connections:
```sh
PID=$(docker inspect --format '{{.State.Pid}}' lab-go-idlepool-client)
docker exec lab-analysis sh -c "
  nsenter --net=/proc/$PID/ns/net ss -tanH state established | awk '{print \$3}' | sort > /tmp/conns-before
  sleep 30
  nsenter --net=/proc/$PID/ns/net ss -tanH state established | awk '{print \$3}' | sort > /tmp/conns-after
  wc -l < /tmp/conns-before; wc -l < /tmp/conns-after
  diff /tmp/conns-before /tmp/conns-after && echo 'same connections'"
```
producing output:
```
16
16
same connections
```
The same 16 local ports appear in both snapshots, so the client went
30s without opening or closing a single connection. The calls are
spread over a fixed set of connections that stay open.

16 is fewer than the 19 counted above. These snapshots weren't taken at
the same moment as that count. Over the gap, a few connections that sat
idle at the bottom of the pool may have been closed by
`IdleConnTimeout` (90s). That's the small, expected source of closes
described at the top. At most it's a few connections every minute or
two, compared with about 110 per second with the default limit.

### Result

Prediction 3 holds. With the idle limit above peak concurrency, the
client dials one connection per call in flight during warmup and then
stops:

| | default (2) | 64 |
|---|---|---|
| connects per 10s (kernel) | 1127 | 0 |
| connects per call | 0.38 | 0 |
| first closes per 10s | 1098 (client) | 0 |
| client `TIME_WAIT` | 7264 | 0 |
| established connections | 7-8 | 16-19, unchanged over 30s |
| p50 / p99 | 27 / 43ms | 27 / 43ms |

### What this means in practice

- **Size the idle limit from peak calls in flight per host, not
  average.** By Little's law, calls in flight = rate × latency. Here
  that's 300/s × 25ms ≈ 8 on average, while the pool settled at 16-19.
  Using a high percentile of latency gets closer: 300/s × 43ms (p99) ≈
  13, about the `peak` the client logged. Then add headroom for bursts.
- **Leave room for the downstream slowing down.** If the backend's
  latency doubles, so does the number of calls in flight. With a limit
  set just at today's peak, the churn comes back exactly when the
  downstream is already struggling, and adds a handshake to the calls.
  The pool only keeps connections the load actually needed, so a
  generous limit (64 here, against a peak of about 19) costs little.
- **The cost of the fix is idle connections, at both ends.** Each
  client instance keeps up to its peak concurrency open to each host.
  For a backend with many client instances, its connection count is
  roughly the sum of those peaks. That usually matters less than
  absorbing 113 handshakes per second, but it's the number to check
  against the backend's own connection limits.
- **How to tell it's fixed, from the outside:** connects per call close
  to 0, a client `TIME_WAIT` count near 0, and the same set of local
  ports across two `ss` snapshots. None of it needs anything from the
  application.
- **The fix doesn't show up in latency here.** On a fast local network,
  churn costs very little latency, which is why it often goes unnoticed.
  The improvement is in connects, closes, `TIME_WAIT` and port usage.
  Latency improves where a handshake is expensive: TLS, or a round trip
  between zones or regions.

### Clean up

```sh
docker compose -f compose.yml down
```
