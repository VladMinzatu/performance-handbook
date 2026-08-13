# Atomics vs. mutex contention, down to the futex

Uses the shared lab infrastructure in [tools/](../tools/README.md), but
only for the `analysis` container's tracing - the two counter containers
here don't need `labnet` or any network reachability at all.

## Background

`sync/atomic` and `sync.Mutex` both let goroutines safely share a piece
of state, and both are usually introduced as roughly interchangeable
tools for "protect this from concurrent access." They aren't
interchangeable underneath, though. An atomic increment
(`atomic.AddInt64`) compiles straight down to a single lock-free CPU
instruction - a `LOCK`-prefixed instruction on amd64, an LSE atomic or
load-linked/store-conditional loop on arm64 - executed entirely in
userspace. There is no kernel involvement, ever, no matter how many
goroutines are hammering the same counter.

`sync.Mutex` is a hybrid. The uncontended fast path is also just a single
CAS - cheap, no syscall. But once a goroutine can't acquire the lock
immediately, Go's runtime has it spin briefly in userspace (an adaptive
spin, only worth attempting if the lock looks likely to be released
soon), and only *after* that gives up and calls into the runtime's
semaphore implementation to actually park the goroutine - which, on
Linux, bottoms out in the `futex(2)` syscall: `FUTEX_WAIT` to sleep until
woken, `FUTEX_WAKE` (called by whoever unlocks) to wake a waiter. This is
the same mechanism glibc's `pthread_mutex_t` uses under the hood, so
what this lab finds isn't Go-specific.

This continues a theme from the last two labs. [Lab 0102](../0102-go-netpoller-epoll/README.md)
showed the netpoller keeps network I/O out of the kernel's way entirely;
[lab 0103](../0103-go-blocking-syscalls-vs-network-io/README.md) showed a
genuine blocking syscall doesn't get that treatment, and costs a real OS
thread per concurrently-blocked goroutine. This lab is the same question
applied to synchronization instead of I/O or scheduling: does contending
for a lock actually reach the kernel, and if so, when and how much?

## Hypotheses

**Prediction 1 - atomics never touch the kernel, at any contention
level.** Tracing `futex` syscalls on `MODE=atomic` should show zero
events, whether `WORKERS` is 1 or 100 - `atomic.AddInt64` has no path
into the kernel at all, so there's nothing for contention to escalate
into.

**Prediction 2 - mutex contention shows up directly as `futex` syscalls,
and the rate tracks contention, not raw operation count.** With
`WORKERS` well below `GOMAXPROCS`, `MODE=mutex` should show few or no
`futex` calls - collisions are rare enough that most `Lock()` calls hit
the uncontended fast path. Once `WORKERS` is pushed well past
`GOMAXPROCS` (this lab's default: `WORKERS=32` against a `GOMAXPROCS`
that will typically be far smaller in a container), far more goroutines
should be simultaneously trying to acquire the same lock than can
possibly be running at once, forcing many of them onto the slow,
kernel-visible path - so the `futex` syscall rate should rise
substantially with `WORKERS`, even though every configuration is doing
the exact same kind of work (incrementing a counter, as fast as
possible).

**Prediction 3 - that kernel round-trip has a real throughput cost.**
`MODE=mutex`'s reported `ops/sec` should be meaningfully lower than
`MODE=atomic`'s at the same `WORKERS`, and the gap should widen as
`WORKERS` grows past `GOMAXPROCS` - more contention means more parking
and waking through the kernel, which is strictly more expensive than the
userspace-only atomic path, on top of the serialization both approaches
already have to pay for (only one goroutine can hold the lock, or land
the winning CAS, at a time either way).

**Prediction 4 (stretch) - the runtime avoids the kernel when it can, so
`futex` calls shouldn't scale 1:1 with contended `Lock()` attempts.**
Because of the adaptive spin, some short-lived contention should resolve
without ever reaching `futex` at all. Comparing the `futex` call rate
against the throughput gap from Prediction 3 at a couple of different
`WORKERS` values should show the relationship isn't perfectly linear -
consistent with the same pattern seen in
[lab 0102](../0102-go-netpoller-epoll/README.md) (netpoller) and
[lab 0103](../0103-go-blocking-syscalls-vs-network-io/README.md) (thread
creation): the Go runtime treats the kernel as a last resort, not a first
one.

## Setup

Build and start both containers - identical image, identical `WORKERS`,
differing only in `MODE`:
```sh
docker compose -f compose.yml up -d --build
```
`counter-atomic` increments via `atomic.AddInt64`; `counter-mutex` via a
`sync.Mutex`-protected increment. Both spawn `WORKERS=32` goroutines and
print `ops/sec` once a second:
```sh
docker logs -f lab-go-counter-atomic
docker logs -f lab-go-counter-mutex
```

To compare at a different `WORKERS` (e.g. below vs. well above
`GOMAXPROCS`, to see contention actually kick in), edit the
`environment:` blocks in `compose.yml` and rebuild:
```sh
docker compose -f compose.yml up -d --build
```

For the `futex` tracing predictions, use the shared analysis container
(see [tools/](../tools/README.md) for the one-time setup). Filtering by
`comm` rather than PID is the reliable option in this environment - PID
filtering was unreliable in earlier labs (see
[lab 0102's experiment 02](../0102-go-netpoller-epoll/experiments/02_epoll_registration_vs_wait.md)
for why):
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
docker compose -f ../tools/analysis/compose.yml exec analysis bash
# inside the analysis container:
bpftrace -e 'tracepoint:syscalls:sys_enter_futex /comm == "counter"/ { @[args.op] = count(); }'
```
The tracepoint's `op` field encodes both the operation (`FUTEX_WAIT`,
`FUTEX_WAKE`, ...) and flag bits (notably `FUTEX_PRIVATE_FLAG`) packed
together, so the raw values won't match the bare `FUTEX_WAIT`/`FUTEX_WAKE`
constants directly - worth confirming empirically which values actually
show up (the same kind of discovery lab 0102 needed for `epoll_wait` vs.
the `epoll_pwait` Go actually calls) rather than assuming them up front.

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down
```
