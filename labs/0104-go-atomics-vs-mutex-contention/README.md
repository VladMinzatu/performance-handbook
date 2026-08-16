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
(see [tools/](../tools/README.md) for the one-time setup). Earlier labs
found PID-based filtering unreliable when sourced via `pgrep` run
*inside* the analysis container (see
[lab 0102's experiment 02](../0102-go-netpoller-epoll/experiments/02_epoll_registration_vs_wait.md)
for why) and fell back to filtering by `comm` instead. Sourcing the PID
from `docker inspect` on the host side instead - the Docker daemon's own
bookkeeping, not a `/proc` scan from inside another container - avoids
that problem and reliably isolates a single container's process, even
with other same-`comm` processes running alongside it:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build

PID=$(docker inspect --format '{{.State.Pid}}' lab-go-counter-atomic)
docker exec lab-analysis sh -c "bpftrace -e 'tracepoint:syscalls:sys_enter_futex /pid == $PID/ { @[args.op] = count(); }'"
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
