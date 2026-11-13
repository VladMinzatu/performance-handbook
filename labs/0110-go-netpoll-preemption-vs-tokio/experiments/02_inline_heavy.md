## Inline computation: tokio stalls, Go doesn't

Tests **Prediction 2** (with the computation inline in the handler,
tokio's fast requests wait for the slow ones, and Go's don't) and
**Prediction 3** (from the outside, the stall shows up as gaps in
`epoll_wait`).

The client keeps sending 200 req/s to `/cheap`, and now also keeps 2
`/heavy` requests in flight, each computing for about 200ms inline in
the handler. With 2 workers per server, that's one computation per
worker at all times. Both servers run the same handler code: the only
difference is whether the runtime can interrupt it.

Start the analysis container and the lab with its defaults, from this
lab's directory. The client starts against the Go server:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
docker compose -f compose.yml up -d --build --force-recreate
```
`--force-recreate` starts both servers fresh, with the tokio server in
its default `HEAVY_MODE=inline`.

### Go: the client's view

Wait for a few 5s reports:
```sh
docker logs --tail 5 lab-go-preempt-client
```
producing output:
```
2026/11/13 18:10:50 target=http://lab-go-preempt-goserver:8080 rate=200/s heavy_conc=2
t=   5s cheap ok=  996 err=  0 p50=8.7ms    p99=30.1ms   max=51.9ms   | heavy ok= 48 mean=206ms
```

`/cheap` still runs at 200 req/s with no errors, but it's no longer
free: p50 went from 400µs without `/heavy` traffic to 8.7ms, and p99
from about 1ms to 30ms. A `/cheap` request now arrives at a server whose
2 `P`s are both busy computing, and it has to wait for one of them to be
preempted. That wait is on the scale of Go's 10ms preemption interval,
not of the 200ms computation.

`/heavy` requests average 206ms, about their calibrated 200ms. They
share the CPU with the `/cheap` requests, but those take so little
CPU that the computation barely slows down.

This is a single 5s window, so p99 and max are less settled than in the
other runs. p99 is a bit above the predicted 20ms; the preemption
signals below show why.

### Go: gaps between epoll waits

```sh
SRV=$(docker inspect --format '{{.Id}}' lab-go-preempt-goserver)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e '
tracepoint:syscalls:sys_enter_epoll_*wait*
/cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${SRV}\")/ {
  if (@last) { @gap_ms = hist((nsecs - @last) / 1000000); @max_gap_ms = max((nsecs - @last) / 1000000); }
  @last = nsecs;
}
END { clear(@last); }'"
```
producing output:
```
Attaching 3 probes...

@gap_ms: 
[0]                  256 |@@@@@@@@@@@@@@                                      |
[1]                    5 |                                                    |
[2, 4)                 6 |                                                    |
[4, 8)                44 |@@                                                  |
[8, 16)              894 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[16, 32)              22 |@                                                   |
[32, 64)               0 |                                                    |
[64, 128)              1 |                                                    |

@max_gap_ms: 65
```

About 1,230 epoll waits in 10s, about 123 per second. That's fewer than
the 200 requests per second arriving, against 2.8 calls per request in
the baseline: each call now picks up about 2 requests that arrived
since the previous one.

The gaps cluster at 8-16ms (894 of them). With both `P`s computing, no
thread runs out of work and goes looking in the netpoller, so it's
`sysmon` that polls it, when nobody has done so for 10ms. The 256 gaps
under 1ms are quick follow-up checks, from a thread that was just
preempted looking for work. So Go keeps noticing new requests about
every 10ms, even with every `P` busy.

The tail is short: 22 gaps of 16-32ms and a single longest one of 65ms
in 10s, about a third of one computation.

### Go: preemption signals

Go preempts a goroutine that has run for more than 10ms by sending its
thread a `SIGURG` (signal 23). Counting the `SIGURG`s generated in the
server's cgroup over 10s shows how often the runtime interrupts the
computation:
```sh
SRV=$(docker inspect --format '{{.Id}}' lab-go-preempt-goserver)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e '
tracepoint:signal:signal_generate
/args.sig == 23 && cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${SRV}\")/ { @sigurg[args.comm] = count(); }'"
```
producing output:
```
Attaching 1 probe...

@sigurg[goserver]: 986
```

986 preemption signals in 10s, about 99 per second, all sent by the Go
runtime inside the server (`args.comm` is the sending thread's name).
That's about 50 per second for each of the 2 computing threads, so each
computation is interrupted roughly every 20ms, not every 10ms. `sysmon`
checks every 10ms and only preempts a goroutine that has been running
for *more* than 10ms. A goroutine that starts just after a check is
under the limit at the next check and gets preempted at the one after,
about 20ms in.

Each preemption is a chance for a waiting `/cheap` request to run. Up to
about 20ms between them, plus the 10ms between netpoller checks, is
consistent with the `/cheap` p99 of 30ms above.

### Go: per-thread CPU

CPU per thread over 5s:
```sh
docker exec lab-analysis sh -c 'pidstat -t -p $(pgrep -x goserver) 5 1' | grep -E 'TID|Average'
```
producing output:
```
18:12:33      UID      TGID       TID    %usr %system  %guest   %wait    %CPU   CPU  Command
Average:        0      9032         -  200.00    0.00    0.00    0.40  200.00     -  goserver
Average:        0         -      9032   99.80    0.00    0.00    0.40   99.80     -  |__goserver
Average:        0         -      9067    0.00    0.00    0.00    0.20    0.00     -  |__goserver
Average:        0         -      9068    0.00    0.00    0.00    0.00    0.00     -  |__goserver
Average:        0         -      9069  100.00    0.00    0.00    0.00  100.00     -  |__goserver
```

The server uses its full 2 cores (200%), on 2 threads at 100% each: the
main thread (Go runs it as an ordinary worker thread) and one other.
The remaining threads are idle. `sysmon`'s polling and signalling don't
register at this resolution.

### tokio: the client's view

Switch the client to the tokio server. `--no-deps` keeps compose from
recreating the servers:
```sh
TARGET=tokio docker compose -f compose.yml up -d --no-deps client
```
Wait for a few 5s reports:
```sh
docker logs --tail 5 lab-go-preempt-client
```
producing output:
```
t=   5s cheap ok=  971 err=  0 p50=104.4ms  p99=239ms    max=295.9ms  | heavy ok= 48 mean=203ms
t=  10s cheap ok= 1026 err=  0 p50=103.8ms  p99=240.3ms  max=276.2ms  | heavy ok= 49 mean=209ms
t=  15s cheap ok=  976 err=  0 p50=98.7ms   p99=217.1ms  max=257ms    | heavy ok= 48 mean=203ms
t=  20s cheap ok= 1018 err=  0 p50=103.4ms  p99=231.2ms  max=282.2ms  | heavy ok= 49 mean=208ms
t=  25s cheap ok=  980 err=  0 p50=115.3ms  p99=289.3ms  max=316.9ms  | heavy ok= 48 mean=204ms
```

The same load, the same handler code and the same 2 workers, and
`/cheap` latency is now on the scale of the computation: p50 of
99-115ms, about half of a `/heavy` call, and p99 of 217-289ms. That's
12x Go's p50 and about 8x its p99.

A `/cheap` request that arrives while both workers are computing waits
until one of them finishes. On average that's half a computation, which
is where the 100ms p50 comes from. p99 and max go past one full
computation (up to about 300ms): occasionally a request waits behind
more than one, for example when a worker picks up a new `/heavy`
request before the `/cheap` one.

There are still no errors, and the request count averages out to 200/s
(1,000 per window), but it swings between 971 and 1,026. Requests held
up at the end of one window complete in the next. Nothing is dropped;
it's all delay.

`/heavy` requests average 203-209ms, the same as on Go. Not being
interrupted didn't make the computation any faster. Go's preemption cost
the computation nothing measurable, and tokio's lack of it cost
everything else on the server about 100ms.

### tokio: gaps between epoll waits

```sh
SRV=$(docker inspect --format '{{.Id}}' lab-go-preempt-tokioserver)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e '
tracepoint:syscalls:sys_enter_epoll_*wait*
/cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${SRV}\")/ {
  if (@last) { @gap_ms = hist((nsecs - @last) / 1000000); @max_gap_ms = max((nsecs - @last) / 1000000); }
  @last = nsecs;
}
END { clear(@last); }'"
```
producing output:
```
Attaching 3 probes...

@gap_ms: 
[0]                   81 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[1]                    5 |@@@                                                 |
[2, 4)                 8 |@@@@@                                               |
[4, 8)                 1 |                                                    |
[8, 16)                0 |                                                    |
[16, 32)               0 |                                                    |
[32, 64)               0 |                                                    |
[64, 128)              0 |                                                    |
[128, 256)            48 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                      |

@max_gap_ms: 224
```

About 140 epoll waits in 10s, 14 per second, against about 2,000 in the
baseline and about 1,230 for Go under the same load. Only 48 of the
gaps are long, at 128-256ms, but together they add up to about 10s: the
server spent almost the entire window without asking the kernel which
sockets had data. The longest gap was 224ms, one computation plus a
little.

The short gaps (95 under 8ms) come right after the long ones. When a
worker finishes a computation it polls, finds every request that came
in meanwhile, serves the `/cheap` ones, and polls again straight away.
That the long gaps are 128-256ms, not around 100ms, suggests the 2
workers tend to finish their computations at about the same time,
so nothing polls in between.

This is the outside signal Prediction 3 expected: a process under load
that goes for a whole computation's length without an epoll wait. Go,
under the same load, never went more than about 30ms without one, apart
from a single gap of 65ms.

### tokio: preemption signals

The same count for the tokio server. Nothing in tokio interrupts a
running task, so this should print no `@sigurg` map at all:
```sh
SRV=$(docker inspect --format '{{.Id}}' lab-go-preempt-tokioserver)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e '
tracepoint:signal:signal_generate
/args.sig == 23 && cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${SRV}\")/ { @sigurg[args.comm] = count(); }'"
```
producing output:
```
Attaching 1 probe...

```

No `@sigurg` map: no preemption signals at all. Nothing in tokio can
interrupt a running task. The kernel still time-slices the worker
threads against other threads on the same cores, but there are none
here, and nothing on the runtime side does it.

### tokio: per-thread CPU

CPU per thread over 5s:
```sh
docker exec lab-analysis sh -c 'pidstat -t -p $(pgrep -x tokioserver) 5 1' | grep -E 'TID|Average'
```
producing output:
```
18:14:19      UID      TGID       TID    %usr %system  %guest   %wait    %CPU   CPU  Command
Average:        0      9011         -  199.80    0.20    0.00    0.00  200.00     -  tokioserver
Average:        0         -      9011    0.00    0.00    0.00    0.00    0.00     -  |__tokioserver
Average:        0         -      9128  100.00    0.00    0.00    0.00  100.00     -  |__tokio-rt-worker
Average:        0         -      9129  100.00    0.00    0.00    0.00  100.00     -  |__tokio-rt-worker
```

The same as Go: 200%, with both worker threads at 100%. From per-thread
CPU alone, a stalled tokio server and a Go server that is still serving
its other requests look identical. Both are computing on every core
they have. Only the epoll gaps and the preemption signals tell them
apart.
