## Churn with the default idle limit

Tests **Prediction 1** (with `MaxIdleConnsPerHost` at its default of 2,
a large share of calls dial a new connection) and **Prediction 2** (the
client is the side that closes). Both are measured from the outside:
the kernel's TCP events for each container's cgroup and the sockets in
the client's network namespace. The client's own log is used only to
cross-check them.

The client sends 300 calls/s to a backend that takes 10-40ms. That's
about 7-8 calls in flight on average, and more during bursts, which is
well above the 2 idle connections the pool keeps.

Start the analysis container and the lab with its defaults, from this
lab's directory:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
docker compose -f compose.yml up -d --build
```
Wait at least 60s before taking the measurements below, so the
`TIME_WAIT` count has time to reach a steady level.

### The client's own view

```sh
docker logs --tail 5 lab-go-idlepool-client
```
producing output:
```
t=  20s ok= 1501 err=  0 p50=27.1ms  p99=43ms    new= 22.0% dials=  574 peak=19
t=  25s ok= 1502 err=  0 p50=27.4ms  p99=43.1ms  new= 23.5% dials=  576 peak=14
t=  30s ok= 1500 err=  0 p50=27.1ms  p99=42.8ms  new= 23.0% dials=  574 peak=14
t=  35s ok= 1498 err=  0 p50=27.2ms  p99=43.1ms  new= 23.9% dials=  570 peak=14
t=  40s ok= 1502 err=  0 p50=27.2ms  p99=42.9ms  new= 23.0% dials=  588 peak=13
```

`new` is the share of calls that got a newly dialed connection, `dials`
is every connection dialed in the 5s window, and `peak` is the most
calls in flight at once.

The rate holds steady at 300 calls/s (about 1500 per 5s window) with no
errors. About 23% of calls, roughly 345 per window, ran on a connection
dialed just for them. The client dialed about 575 connections per
window, though, which is about 115 per second and 0.38 per call.

The difference between the two, about 230 dials per window (40% of
all dials), is dials that no call ended up using. A call with no idle
connection waits for one to be returned and starts a dial at the same
time. Often a busy connection comes back first, so the call takes that
one instead. The dial still completes, and its new connection then
goes into the idle pool, or is closed straight away if 2 connections
are already idle. So the idle limit causes churn in two ways: busy
connections closed when they're returned to a full pool, and dials
nobody used.

`peak` is 13-19 calls in flight, against an average of about 8. Each
burst dials connections that the pool can't keep once the burst passes.

Latency isn't where this hurts here: p50 is 27ms, close to the
backend's average service time of 25ms. On this network a TCP handshake
takes well under a millisecond. Between zones or regions, or with TLS,
it would add one or more round trips to about a quarter of the calls.

### The backend's view

```sh
docker logs --tail 5 lab-go-idlepool-backend
```
producing output:
```
t=  45s requests= 1501 accepted=  592 open=9
t=  50s requests= 1502 accepted=  573 open=11
t=  55s requests= 1500 accepted=  569 open=7
t=  60s requests= 1501 accepted=  591 open=6
t=  65s requests= 1498 accepted=  558 open=9
```
`accepted` (per 5s) should match the client's `dials`, and `open` is
the number of connections open at that moment.

`accepted` is 558-592 per window, the same as the client's `dials`. The
backend sees every one of those dials as a new connection, including
the ones the client never used for a call. Only 6-11 connections are
open at any moment, about the number of calls in flight. Few
connections survive long enough to stay idle in the pool, so nearly
every open connection is serving a call.

### Connect rate, from the kernel

`tcp_connect` fires once for every outgoing connection, inside the
`connect()` call, so it's counted against the cgroup of the container
that dials. Count it in the client's cgroup over 10s:
```sh
CLI=$(docker inspect --format '{{.Id}}' lab-go-idlepool-client)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e 'kprobe:tcp_connect /cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${CLI}\")/ { @connects = count(); }'"
```
producing output:
```
Attaching 1 probe...

@connects: 1127
```
That's 113 connects per second, matching the client's own `dials`
count (about 115/s) and the backend's `accepted`. The kernel's count
needs nothing from the application, so it works the same on a service
you can't change or add logging to. 113 connects/s against 300 calls/s
is 0.38 connects per call. A client that reuses its connections would
show close to 0. That ratio is the signal to look for.

### Which side closes first

The side that closes a connection first moves it from `ESTABLISHED` to
`FIN_WAIT1` (state `4` in the `sock:inet_sock_set_state` tracepoint).
That happens inside its `close()` call, so it's counted against the
right container's cgroup. That side later holds the `TIME_WAIT` entry.
Count those transitions in both containers over 10s:
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

@closed_first[client]: 1098
```
The client closed first 1098 times and the backend never did, since a
side with no entry closed first zero times. 1098 closes in 10s is
about 110 per second, the same as the connect rate: every connection
the client opens, the client also closes, and it closes first.

This rules out the server's keep-alive settings. If the backend were
closing connections, for example because of a short idle timeout or a
limit on requests per connection, `@closed_first[backend]` would show
up here. The churn comes from the client's pool returning connections
and then closing them.

### Sockets by state

Established and `TIME_WAIT` sockets in each container's network
namespace:
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
lab-go-idlepool-client established        7
lab-go-idlepool-client time-wait       7264
lab-go-idlepool-backend established       8
lab-go-idlepool-backend time-wait         0
```
At a steady churn rate, the client's `TIME_WAIT` count should settle at
about connects per second × 60.

The client has 7264 sockets in `TIME_WAIT`. Linux keeps each one for
60s, so that's about 121 closes per second. The connect rate measured
above, 113/s, predicts about 6800. The two are within about 7%, which
is reasonable for rates measured over different windows that move
between about 112/s and 118/s. The backend has none, as expected for
the side that didn't close first.

Each of those 7264 sockets holds one local port for the same
destination (`lab-go-idlepool-backend:8080`). The default
`ip_local_port_range` has about 28,000 ports, so this client is using
about a quarter of them. That's at just 300 calls/s to a backend that
answers in 25ms. About 4× the rate, or longer-running calls that put
more in flight, would use up the range, and dials would start failing
with `cannot assign requested address`.

Only 7-8 connections are established at a time, on both sides, which
again matches the number of calls in flight. The two counts were taken
a moment apart, so they differ by one.

### Result

Both predictions hold.

- **Prediction 1:** 23% of calls ran on a newly dialed connection, and
  the client dialed 0.38 connections per call, about 113/s at 300
  calls/s. `TIME_WAIT` settled in the thousands (7264), at about the
  connect rate × 60. About 40% of the dials were never used by the call
  that started them, which is churn on top of what the idle limit
  causes directly.
- **Prediction 2:** every first close came from the client and none
  from the backend, so the client holds all the `TIME_WAIT` entries.
  The cause is the client's `MaxIdleConnsPerHost`, not the server.

### Clean up

Leave the lab running for the next experiment, or stop it:
```sh
docker compose -f compose.yml down
```
