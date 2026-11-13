## Baseline without CPU-bound work

Tests **Prediction 1** (without CPU-bound work, both runtimes are
equally fast). With no `/heavy` traffic, the only difference between
the two servers is the language, the HTTP stack and the runtime. If
`/cheap` latency is about the same on both, the differences in the
later experiments come from how each runtime handles the computation.

This run also records two reference points for the later experiments:
how often each server calls `epoll_wait` when nothing is stalling it,
and what its threads look like when they're mostly idle.

Start the analysis container and the lab, from this lab's directory,
with the client sending only `/cheap` traffic to the Go server:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
HEAVY_CONC=0 docker compose -f compose.yml up -d --build --force-recreate
```
`--force-recreate` starts both servers fresh, so nothing from an earlier
run carries over.

### Both servers do the same work per `/heavy` call

Each server calibrates its loop at startup. The loop counts should be
within a few percent of each other, so a `/heavy` call is the same
amount of work on both servers.

```sh
docker logs lab-go-preempt-goserver
docker logs lab-go-preempt-tokioserver
```
producing output:
```
2026/11/13 11:08:16 GOMAXPROCS=2 HEAVY_MS=200 iters=73276459

workers=2 HEAVY_MS=200 HEAVY_MODE=inline iters=72970624
```

The loop counts differ by 0.4% (73.3M vs. 73.0M iterations), so the
same arithmetic compiles to about the same speed in Go and Rust: about
365M iterations per second on one core. A `/heavy` call is the same
amount of work on both servers, so any later difference in how long it
takes, or in what it does to other requests, comes from the runtime.

### Go: the client's view

Wait for a few 5s reports:
```sh
docker logs --tail 5 lab-go-preempt-client
```
producing output:
```
t=  35s cheap ok= 1000 err=  0 p50=400µs    p99=1ms      max=5.4ms    | heavy ok=  0 mean=0s
t=  40s cheap ok= 1000 err=  0 p50=400µs    p99=1ms      max=7.9ms    | heavy ok=  0 mean=0s
t=  45s cheap ok= 1000 err=  0 p50=400µs    p99=1.1ms    max=11.7ms   | heavy ok=  0 mean=0s
t=  50s cheap ok= 1000 err=  0 p50=400µs    p99=900µs    max=1.6ms    | heavy ok=  0 mean=0s
t=  55s cheap ok= 1000 err=  0 p50=400µs    p99=800µs    max=17.1ms   | heavy ok=  0 mean=0s
```

The client holds 200 req/s (1000 per 5s window) with no errors. p50 is
400µs and p99 is 0.8-1.1ms, the cost of a round trip over the Docker
network and a trivial handler. `max` varies from 1.6ms to 17ms between
windows: single slow requests, not a pattern, since p99 doesn't move.
These are most likely noise from the VM and the scheduler, and they
set the floor for what counts as a real effect later.

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
[0]                 3597 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[1]                    6 |                                                    |
[2, 4)               406 |@@@@@                                               |
[4, 8)              1554 |@@@@@@@@@@@@@@@@@@@@@@                              |
[8, 16)                6 |                                                    |
[16, 32)               2 |                                                    |

@max_gap_ms: 26
```

The server called an epoll wait about 5,600 times in 10s, about 2.8
times per request. The histogram has two peaks:

- **4-8ms (1,554 gaps):** the interval between requests. At 200 req/s
  one arrives every 5ms, and the server goes back to waiting for the
  next one in between. So when nothing is stalled, the gap tracks the
  request rate.
- **Under 1ms (3,597 gaps):** several calls in quick succession. Go's
  scheduler checks the netpoller from more than one place: a thread
  looking for work polls without blocking before it goes to sleep, and
  one thread then blocks in it. A request that wakes the server is
  followed by a few such quick checks.

The longest gap was 26ms, and only 8 of the 5,600 gaps were over 8ms.
That's the healthy baseline to compare against: gaps at the request
interval, with occasional outliers of a few tens of milliseconds.

### Go: per-thread CPU

CPU per thread over 5s:
```sh
docker exec lab-analysis sh -c 'pidstat -t -p $(pgrep -x goserver) 5 1' | grep -E 'TID|Average'
```
producing output:
```
11:10:27      UID      TGID       TID    %usr %system  %guest   %wait    %CPU   CPU  Command
Average:        0      8260         -    2.00    2.00    0.00    0.20    3.99     -  goserver
Average:        0         -      8260    0.60    0.60    0.00    0.20    1.20     -  |__goserver
Average:        0         -      8304    0.20    0.40    0.00    0.80    0.60     -  |__goserver
Average:        0         -      8305    0.00    0.00    0.00    0.00    0.00     -  |__goserver
Average:        0         -      8306    0.40    0.60    0.00    0.20    1.00     -  |__goserver
Average:        0         -      8367    0.80    0.40    0.00    0.40    1.20     -  |__goserver
```

The whole process uses about 4% of one core. It has 5 threads for 2 `P`s:
Go creates OS threads as it needs them, for example `sysmon` runs on its
own thread without a `P`, and a thread that has gone idle is kept for
reuse. The work is spread thinly across them, with none above 1.2%. All
threads are named `goserver`, so from the outside you can't tell which
one is doing what.

### tokio: the client's view

Switch the client to the tokio server. `--no-deps` keeps compose from
recreating the servers:
```sh
TARGET=tokio HEAVY_CONC=0 docker compose -f compose.yml up -d --no-deps client
```
Wait for a few 5s reports:
```sh
docker logs --tail 5 lab-go-preempt-client
```
producing output:
```
t=   5s cheap ok=  998 err=  0 p50=300µs    p99=500µs    max=2.7ms    | heavy ok=  0 mean=0s
t=  10s cheap ok= 1001 err=  0 p50=300µs    p99=1.2ms    max=5.7ms    | heavy ok=  0 mean=0s
```

The same load against tokio: 200 req/s, no errors, p50 of 300µs and p99
of 0.5-1.2ms. The 100µs difference in p50 from Go is the size of the
client's rounding step, and p99 overlaps. For a trivial handler the two
HTTP stacks are equally fast.

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
[0]                   69 |@@                                                  |
[1]                    7 |                                                    |
[2, 4)               444 |@@@@@@@@@@@@@@@                                     |
[4, 8)              1519 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[8, 16)                5 |                                                    |
[16, 32)               1 |                                                    |

@max_gap_ms: 19
```

About 2,000 epoll waits in 10s, about one per request, against Go's 2.8.
Almost all gaps (1,519) are 4-8ms, the interval between requests, and
only 69 are under 1ms. Tokio only calls the I/O driver when a worker
runs out of tasks (or every 61 tasks), and it blocks in it straight
away, so there are none of Go's quick non-blocking checks. The longest
gap was 19ms.

The two servers reach the same baseline in different ways, but what
matters for the next experiments is the same for both: when nothing is
stalled, gaps sit at the request interval and the longest is a few tens
of milliseconds.

### tokio: per-thread CPU

CPU per thread over 5s:
```sh
docker exec lab-analysis sh -c 'pidstat -t -p $(pgrep -x tokioserver) 5 1' | grep -E 'TID|Average'
```
producing output:
```
11:13:03      UID      TGID       TID    %usr %system  %guest   %wait    %CPU   CPU  Command
Average:        0      8265         -    0.60    1.00    0.00    0.00    1.60     -  tokioserver
Average:        0         -      8265    0.00    0.00    0.00    0.00    0.00     -  |__tokioserver
Average:        0         -      8364    0.60    1.00    0.00    0.00    1.60     -  |__tokio-rt-worker
Average:        0         -      8365    0.20    0.00    0.00    0.00    0.20     -  |__tokio-rt-worker
```

About 2% of one core in total, on exactly 3 threads: the main thread and
the 2 workers. The main thread is at 0%. It runs the accept loop
(`block_on` runs its future on the calling thread, not on a worker), and
the client's keep-alive connections were all accepted at startup.

The 2 workers aren't sharing the load evenly: one did almost all of it
(1.6%), the other almost none (0.2%). At this load a worker finishes a
request long before the next one arrives, so whichever worker is
waiting on the I/O driver picks up the next request, and the other
stays parked. That's fine here, but it means the tokio server only
needs one of its two workers for this traffic, and has the second spare
for whatever else comes in.

### Conclusion

**Prediction 1 holds.** Without CPU-bound work, both servers serve
200 req/s with no errors, a p50 of 300-400µs and a p99 of about 1ms,
using a few percent of one core. They also do the same amount of work
per `/heavy` call, to within 0.4%. So the differences in the next
experiments come from how each runtime handles a long computation, not
from Go or Rust code being faster.

It also sets the baseline for the outside signal. Unstalled, both
servers call an epoll wait at least once per request interval (5ms),
and the longest gap in 10s is 19-26ms. A stalled runtime should show
gaps far beyond that.
