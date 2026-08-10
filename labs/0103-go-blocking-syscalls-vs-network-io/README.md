# Blocking syscalls vs. network I/O: not all blocking is equal

Uses the shared lab infrastructure in [tools/](../tools/README.md). The
`analysis` container is only needed for the syscall-tracing prediction
(2) - the goroutine/thread counts come straight from each worker's own
logs and `/proc`.

## Background

We saw in the previous lab that a goroutine
blocked on network I/O costs no OS thread at all - the netpoller
multiplexes thousands of them onto a shared `epoll` instance, and the
runtime's own thread count stays flat regardless of how many goroutines
are parked in `Read`. It's tempting to generalize that into "blocking a
goroutine is cheap in Go, full stop" - but that's specifically a
network-I/O story, because it relies on the netpoller's special
integration with the scheduler (non-blocking fds, `epoll_ctl`/`epoll_wait`,
`gopark`/`netpollready`).

A *genuine* blocking syscall - one where the kernel really does suspend
the calling thread with no way for userspace to be notified when it's
ready, and nothing in the runtime rewrites it into a non-blocking,
poller-integrated call - doesn't get that treatment. The calling
goroutine's underlying OS thread (M) is really stuck in the kernel for
the duration. Go's scheduler compensates by detaching the `P` that M was
running (`entersyscall`, and `sysmon`'s periodic `retake` if the syscall
runs long), so other goroutines aren't blocked behind it - but making that
`P` productive again means handing it to *some* M, and if every existing
M is itself stuck in a syscall, the only option is to create a new OS
thread. Do that for thousands of concurrently-blocked goroutines, and -
unlike the netpoller case - thread count should climb right along with
them.

This lab isolates the mechanism directly: the same number of goroutines,
blocking for the same wall-clock duration on every iteration, differing
only in *how* they block - a runtime timer (`time.Sleep`, no syscall at
all), a raw blocking syscall (`syscall.Nanosleep`, called directly instead
of through `time.Sleep`), or a network round trip (the netpoller path
from the previous lab, via a companion delay-server). Same apparent "blocking" from
the goroutine's point of view; three different costs underneath.

## Hypotheses

**Prediction 1 - a real blocking syscall costs threads roughly in step
with how many goroutines are concurrently blocked in one; a network read
blocked for the same duration doesn't.** With `WORKERS=2000` goroutines
each blocking for `WORK_MS=200ms` per iteration, `MODE=syscall`'s OS
thread count (`/proc/1/status`'s `Threads:`, or `ls /proc/1/task | wc -l`)
should climb well above `GOMAXPROCS` plus a small constant - potentially
into the hundreds or low thousands, tracking how many goroutines are
simultaneously stuck in `nanosleep` at any instant. `MODE=netpoll`, with
the same `WORKERS` and the same `WORK_MS` (spent blocked on a network
`Read` waiting for `delay-server`'s deliberately delayed reply instead),
should reproduce the previous lab's flat result - thread count staying near
`GOMAXPROCS` plus a small constant regardless of `WORKERS`. `MODE=timer`
should look the same as `MODE=netpoll` on this metric, for an even more
basic reason: `time.Sleep` never calls into the kernel to block at all,
so there's no syscall to tie up a thread with in the first place.

**Prediction 2 - the mechanism is delayed P handoff via sysmon's retake,
not immediate thread-per-syscall creation.** Go doesn't spin up a thread
the instant a goroutine enters a slow syscall - `sysmon` has to first
notice, on its own periodic sweep, that a `P` has been sitting in syscall
state too long and forcibly retake it before a new M can be created for
it. Tracing `clone`/`clone3` syscalls on the `MODE=syscall` worker's PID
while `WORKERS` ramps up should show new-thread creation roughly tracking
the growth in concurrently-blocked goroutines - in sharp contrast to lab
0102's Prediction 3, which found *zero* `clone` calls even during a burst
of thousands of simultaneous network wakeups on the same kind of
workload.

**Prediction 3 (stretch) - thread growth has a real cost and a real
ceiling that network I/O and timers never approach.** Each new M is a
genuine OS thread - kernel scheduling entity, its own stack. Comparing
`MODE=syscall` against `MODE=netpoll`/`MODE=timer` at the same `WORKERS`
should show materially higher memory usage (`docker stats`, or
`/proc/1/status`'s `VmRSS`) for `MODE=syscall`, growing with `WORKERS` in
a way the other two modes don't. Pushed far enough, `MODE=syscall` should
be reachable to a real, discoverable limit: the Go runtime's hard-coded
default cap of 10,000 OS threads per process (`runtime: program exceeds
10000-thread limit`, a fatal error) - something `MODE=netpoll` and
`MODE=timer` structurally can't hit no matter how many goroutines are
waiting, since neither one ties up a thread per waiting goroutine at all.

## Setup

Build and start the companion delay-server and all three workers, joined
to `labnet`:
```sh
docker compose -f compose.yml up -d --build
```
All three workers run `WORKERS=2000` goroutines, each blocking for
`WORK_MS=200ms` per iteration - identical load, different mechanism:
- `worker-timer` (`MODE=timer`) - `time.Sleep(200ms)` per iteration.
- `worker-syscall` (`MODE=syscall`) - a raw `syscall.Nanosleep(200ms)`
  call per iteration, bypassing `time.Sleep`'s runtime-timer path.
- `worker-netpoll` (`MODE=netpoll`) - one held connection to
  `delay-server`, writing a byte and blocking on `Read` for the reply,
  which `delay-server` deliberately delays by `DELAY_MS=200ms` before
  sending.

Watch each worker's own goroutine count:
```sh
docker logs -f lab-go-worker-timer
docker logs -f lab-go-worker-syscall
docker logs -f lab-go-worker-netpoll
```

Check real OS thread counts directly (each worker binary runs as PID 1 in
its container):
```sh
docker exec lab-go-worker-timer    sh -c 'grep Threads /proc/1/status'
docker exec lab-go-worker-syscall  sh -c 'grep Threads /proc/1/status'
docker exec lab-go-worker-netpoll  sh -c 'grep Threads /proc/1/status'
```

To compare against different `WORKERS`/`WORK_MS` values, edit the
`environment:` blocks in `compose.yml` and rebuild:
```sh
docker compose -f compose.yml up -d --build
```

For Prediction 2, trace `clone`/`clone3` on the `worker-syscall` process
from the shared analysis container (see [tools/](../tools/README.md) for
the one-time `labnet`/`analysis` setup). Filtering by `comm` rather than
PID is the reliable option here - PID-based filtering was unreliable in
this environment (see
[lab 0102's experiment 02](../0102-go-netpoller-epoll/experiments/02_epoll_registration_vs_wait.md)
for why):
```sh
docker compose -f ../tools/analysis/compose.yml exec analysis bash
# inside the analysis container:
bpftrace -e 'tracepoint:syscalls:sys_enter_clone /comm == "worker"/ { @ = count(); } tracepoint:syscalls:sys_enter_clone3 /comm == "worker"/ { @ = count(); }'
```

For Prediction 3, compare memory footprints directly:
```sh
docker stats --no-stream lab-go-worker-timer lab-go-worker-syscall lab-go-worker-netpoll
```

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down
```
