# Preemptive vs. cooperative scheduling: a CPU-bound handler stalls tokio, not Go

Uses the shared lab infrastructure in [tools/](../tools/README.md). The
`analysis` container's `bpftrace` shows the stall from the outside,
without any help from the servers' own code.

## Background

Go and Rust's tokio solve the same problem the same way at the bottom:
many connections, few OS threads, and one `epoll` instance telling the
runtime which sockets are ready. Each runtime runs a small pool of
worker threads (Go's `P`s, `GOMAXPROCS` of them; tokio's workers, one
per core by default). The pool runs lightweight tasks and parks those
waiting on a socket until `epoll` says the socket is ready. The
difference is what happens when a task *doesn't* wait: when it just
computes.

**Go preempts.** A goroutine is a stackful task: it has its own stack,
so the runtime can stop it at almost any instruction and resume it later.
Since Go 1.14 it does: the `sysmon` thread checks the running goroutines
about every 10ms, and one that has run longer than that gets a `SIGURG`.
The signal handler saves its state and puts it back on a run queue. That
frees the `P` for other goroutines. `sysmon` also polls `epoll` itself if
no thread has done so for more than 10ms, so newly readable sockets
are noticed even when every `P` is busy computing. A CPU-bound request
in Go slows every other request a little, because they now share CPU.
It doesn't block them.

**Tokio cooperates.** A tokio task is a future: a state machine that the
compiler turns `async fn` code into. A worker runs a task by calling its
`poll` method, and the task gives the thread back only by returning from
`poll`, which happens at an `.await` that isn't ready yet. Between
`.await`s, the task owns the worker thread. The same worker threads also
drive the I/O driver: they call `epoll_wait` when they run out of tasks,
and every 61 tasks otherwise. A task that computes for 200ms without an
`.await` holds its worker for 200ms. Tasks queued on that worker wait,
and nothing on that thread checks `epoll`. When every worker is held this
way, the whole runtime stops: requests sit unread in the kernel's socket
buffers until a worker finishes. Tokio's *cooperative budget* doesn't
help. It makes a task yield after about 128 operations on tokio's own
resources (sockets, channels, timers), and a pure computation touches
none of them.

The tokio docs' guidance is to keep the time between `.await`s
under about 10-100µs, and to move anything longer off the workers:

- `tokio::task::spawn_blocking` runs a closure on a separate pool of
  threads (up to 512 by default) and returns a future the handler can
  `.await`. The OS scheduler, which does preempt, shares the CPU between
  those threads and the workers.
- `tokio::task::block_in_place` turns the current worker into a blocking
  thread and moves its other tasks to a new worker (multi-thread runtime
  only).
- A dedicated pool such as `rayon` for parallel CPU work.

The work that causes this in real services is ordinary handler code
that's easy to forget is expensive: password hashing (`bcrypt`,
`argon2`) in login handlers, (de)serializing large JSON or protobuf
payloads, compression, image resizing, regex over large inputs, and
synchronous calls to libraries that block (`std::fs`, a sync database
driver, `std::thread::sleep`). It shows up in Rust services built on
`axum`, `hyper`, `tonic` or `actix-web`. The same problem in Node.js is
called *event loop lag*, with a single loop where tokio has one per
worker.

The failure doesn't look like overload. CPU usage is high but not
necessarily at the limit, error rates stay at zero, and the expensive
endpoint performs fine. Fast, unrelated endpoints get slow: their
latency jumps to about the duration of the expensive call, and in steps
rather than smoothly. From the outside, the giveaway is that **`epoll`
stops being called.** A healthy runtime under load calls `epoll_wait`
every few milliseconds, from some thread. A stalled one goes for as long
as the expensive call takes without calling it at all. The longest gap
between `epoll_wait` calls across the process approximates the runtime's
worst delay in noticing new work: about 10ms for Go, whose `sysmon`
polls on that interval, and the length of the longest computation for a
stalled tokio runtime.

Inside the process, `tokio-console` and tokio's runtime metrics show the
same thing as long task poll times. Go's `runtime/metrics` has the
equivalent in `/sched/latencies:seconds`.

Go's design has a price for this. Each goroutine needs its own stack, so
an idle connection costs more memory than an idle future. Preemption
needs signals and safe points. And fair time slices don't make the
expensive request any faster; they just stop it from blocking the
others.

## Hypotheses

Both servers run with 2 worker threads on 2 dedicated cores. A client
sends 200 req/s to `/cheap`, which returns at once, and keeps 2 requests
in flight to `/heavy`, which computes for about 200ms. That's one
expensive request per worker, all the time.

**Prediction 1: without CPU-bound work, both runtimes are equally
fast.** With no `/heavy` traffic, `/cheap` should have a p99 of about
1ms on both servers. Any difference later comes from how each runtime
handles the computation, not from a slower HTTP stack.

**Prediction 2: with the computation inline in the handler, tokio's fast
requests wait for the slow ones, and Go's don't.** On the tokio server
(`HEAVY_MODE=inline`), `/cheap` latency should jump to the scale of a
`/heavy` call: p50 around half of one (about 100ms, the average
remaining time of the computation a request arrives behind) and p99 of
about 200ms. There should be no errors and no drop in `/cheap`
throughput once the backlog is served. On the Go server, with the same
handler code and the same 2 workers, `/cheap` p50 should stay at a few
milliseconds and p99 around 20ms, set by the 10ms preemption and `epoll`
polling interval: about 10x lower than tokio's.

**Prediction 3: from the outside, the stall shows up as gaps in
`epoll_wait`.** Counting the time between consecutive `epoll_wait` calls
across all the server's threads, the inline tokio server should
regularly go about 200ms without calling it. The Go server's gaps should
cluster at about 10ms, `sysmon`'s polling interval, with rare outliers of
a few tens of milliseconds. Per-thread CPU doesn't tell them apart: in
both servers, the 2 worker threads are at about 100%.

**Prediction 4: moving the computation to `spawn_blocking` fixes tokio,
and beats Go.** With `HEAVY_MODE=blocking`, the same load should bring
tokio's `/cheap` p99 back to about 1-2ms, the same as with no `/heavy`
traffic at all, and its `epoll_wait` gaps under about 10ms. The 2 cores
are still fully busy computing, now on 2 extra threads from the blocking
pool, with the workers nearly idle. The difference is that the kernel,
not the task, decides when the workers get to run, and the kernel
preempts in well under a millisecond when a worker wakes up, where Go's
runtime waits for its 10ms timer.

## Setup

Start the analysis container and the lab, from this lab's directory:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
docker compose -f compose.yml up -d --build
```
- `lab-go-preempt-goserver` - the Go server, on `net/http`. `GET /cheap`
  returns a short response at once. `GET /heavy` runs a fixed amount of
  arithmetic (a hash loop) inline in the handler and then responds. The
  loop count is calibrated at startup to take `HEAVY_MS` (default 200)
  on an idle core, so both servers do the same work. `GOMAXPROCS` is set
  from `WORKERS` (default 2), and the container is pinned to 2 cores with
  `cpuset` (`GO_CPUS`, default `0-1`), so no CPU quota throttling gets
  mixed into the results.
- `lab-go-preempt-tokioserver` - the same server in Rust, on `tokio`'s
  multi-thread runtime with `hyper`. `worker_threads` is set from
  `WORKERS` (default 2), with its own 2 cores (`TOKIO_CPUS`, default
  `2-3`) and the same calibrated loop. `HEAVY_MODE` picks where `/heavy`
  runs it: `inline` (default) in the handler, or `blocking` inside
  `tokio::task::spawn_blocking`. Worker and blocking-pool threads are
  both named `tokio-rt-worker`.
- `lab-go-preempt-client` - the load generator, in Go. `TARGET` (`go` or
  `tokio`, default `go`) picks the server. It sends `RATE` (default 200)
  `/cheap` requests per second, open loop: requests start on schedule
  whether or not earlier ones have finished. Separately, it keeps
  `HEAVY_CONC` (default 2) `/heavy` requests in flight, closed loop: each
  finished one is replaced right away. Both use keep-alive connections,
  with an idle pool large enough that connections are never re-dialed.
  It waits for the server to answer before starting. Pinned to its own
  cores (`CLIENT_CPUS`, default `4-5`). Logs every 5s: `/cheap` count,
  errors, p50/p99/max, and `/heavy` count and mean latency.

The default pinning needs at least 6 cores. All three are on `labnet`.
Observation commands used in the experiments:
```sh
# the client's view of each endpoint
docker logs --tail 5 lab-go-preempt-client

# container ID, for bpftrace's cgroup filter (or the Go server's)
SRV=$(docker inspect --format '{{.Id}}' lab-go-preempt-tokioserver)

# gaps between consecutive epoll waits across the whole server, over 10s.
# The wildcard covers epoll_wait, epoll_pwait and epoll_pwait2, whichever
# exist: Go uses epoll_pwait, and arm64 has no epoll_wait syscall.
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e '
tracepoint:syscalls:sys_enter_epoll_*wait*
/cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${SRV}\")/ {
  if (@last) { @gap_ms = hist((nsecs - @last) / 1000000); @max_gap_ms = max((nsecs - @last) / 1000000); }
  @last = nsecs;
}
END { clear(@last); }'"

# per-thread CPU: which threads are doing the computing
docker exec lab-analysis sh -c 'ps -L -o tid,comm,pcpu,stat -p $(pgrep -x tokioserver)'
```
A process that's idle also goes long stretches without calling
`epoll_wait`, because it's blocked inside one waiting for work. The gap
only measures a stall under load, which is why the client keeps
`/cheap` traffic flowing during every measurement.

To switch the client to the other server, or change the load, set the
knob when recreating the client. `--no-deps` matters: without it,
compose also recreates any server whose settings differ from the
current environment, which silently resets `HEAVY_MODE` to `inline`.
```sh
TARGET=tokio docker compose -f compose.yml up -d --no-deps client
TARGET=go HEAVY_CONC=0 docker compose -f compose.yml up -d --no-deps client
```
To move tokio's computation to `spawn_blocking`, recreate its server.
The client keeps running and reconnects:
```sh
HEAVY_MODE=blocking docker compose -f compose.yml up -d tokioserver
```

## Experiments

See [Experiments directory](./experiments):
1. [Baseline without CPU-bound work](./experiments/01_baseline.md) -
   Prediction 1.
2. [Inline computation: tokio stalls, Go doesn't](./experiments/02_inline_heavy.md) -
   Predictions 2 and 3.
3. [Moving the computation to spawn_blocking](./experiments/03_spawn_blocking.md) -
   Prediction 4.

## Tear down

```sh
docker compose -f compose.yml down
```
To stop the analysis container as well:
```sh
docker compose -f ../tools/analysis/compose.yml down
```
