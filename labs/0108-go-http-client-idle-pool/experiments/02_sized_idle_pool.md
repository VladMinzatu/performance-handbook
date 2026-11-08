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
The first line shows the startup setting. The first window should show
the warmup dials, about as many as the peak number of calls in flight.
After that, `new` and `dials` should be about 0.

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

### Clean up

```sh
docker compose -f compose.yml down
```
