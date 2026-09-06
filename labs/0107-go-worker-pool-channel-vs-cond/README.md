# Worker-pool queues: a buffered channel vs. a sync.Cond queue

Uses the shared lab infrastructure in [tools/](../tools/README.md), but
only for the `analysis` container's tracing/profiling, if the
experiments need it - none of this lab's containers need `labnet` or
any network reachability at all.

## Background

A bounded producer/consumer queue - the backbone of a worker pool - can be
built two ways in Go: hand a buffered channel to producers and consumers
directly, or build the bounded buffer yourself (a ring buffer behind a
`sync.Mutex`) and use `sync.Cond` to block producers when it's full and
consumers when it's empty. Built carelessly - one shared condition
variable, woken with `Broadcast` - the second option pays a real
thundering-herd cost: every wakeup rouses every blocked goroutine, and all
but one immediately find the condition still doesn't hold for them and go
back to sleep. Built carefully - two condition variables, `notFull` and
`notEmpty`, each woken with `Signal` - that specific waste goes away: a
Push only ever wakes a blocked Pop, and vice versa, one wakeup per item.

That still isn't the same thing as a channel, though. A blocked
`Cond.Wait()` doesn't just get told "your turn" and carry on - waking up
only ends the wait; the goroutine still has to re-acquire the mutex before
it can act, and the correct usage pattern (`for !predicate() { cond.Wait() }`)
re-checks the condition once it does, in case something else changed it
first. A channel's blocked send/receive has no equivalent second step: the
runtime completes the handoff as part of waking the other side, so the
woken goroutine finds the work already done, not just permission to go
check for itself. That's the structural difference this lab isolates,
once the sloppy "one condvar plus Broadcast" version - and its thundering
herd - is off the table.

It's also worth not assuming which side actually reaches the kernel more
often. `sync.Cond` looks like the more "OS-flavored" primitive, but its
blocking path is pure Go-runtime scheduling (parking and waking a
goroutine) with no syscall in the common case. A channel's internal lock
protecting its buffer and wait queues, on the other hand, is a low-level
runtime mutex that spins first but can fall back to a real futex syscall
under contention - a different lock than the one `sync.Mutex` uses, with
different fallback behavior. Which mechanism actually makes more syscalls
under load is a measurement question, not something to assume from how
each API looks on the surface.

## Hypotheses

**Prediction 1 - bulk throughput should be similar between the two
mechanisms.** Both `workerpool-channel` and `workerpool-cond` implement a
correctly bounded queue doing the same logical amount of work at the same
`PRODUCERS`/`CONSUMERS`/`QUEUE_CAP` - unlike a naive Broadcast-based
design, there's no structural source of wasted work here, so
`produced/sec` and `consumed/sec` should land close together for both,
not show the kind of large gap a careless implementation would.

**Prediction 2 - the cond queue should show roughly double the mutex
traffic per blocked transfer.** Every time `Cond.Wait()` actually blocks,
it unlocks the mutex to sleep and re-locks it on waking, on top of the
lock/unlock pair the caller already does around the whole operation - so
a blocked Push or Pop touches the mutex twice where a channel's internal
lock is touched once per send/receive, blocked or not. This should be
visible directly: a mutex/lock profile (or a lock-call trace) taken while
both containers run under the same contention level should show a higher
lock-acquisition count per item transferred for `workerpool-cond` than
for `workerpool-channel`.

**Prediction 3 - the cond queue's tail latency should be worse under
contention than its median suggests.** After `notEmpty.Signal()` wakes a
consumer, that consumer still has to win the mutex back before it can
actually pop - if a producer (or another consumer finishing its own turn)
grabs the mutex first, the woken goroutine re-checks its predicate, finds
nothing to do yet, and waits again. A channel's handoff has no equivalent
"woken but still has to fight for it" step. Shrinking `QUEUE_CAP` relative
to `PRODUCERS`/`CONSUMERS` (more contention, more blocking) should widen
the p99-vs-median latency gap for `workerpool-cond` more than for
`workerpool-channel`, even where median throughput looks similar.

**Prediction 4 (stretch, counterintuitive) - under heavy contention, the
channel implementation may issue more real futex syscalls than the cond
one, not fewer.** If the channel's internal lock spins out and falls back
to blocking, that fallback is a kernel-level futex wait/wake - while
`sync.Mutex` contention is resolved by parking the goroutine at the Go
scheduler level, which doesn't need a syscall per contention event. A
syscall-level trace (e.g. counting `futex` calls per container over a
fixed window) at high `PRODUCERS`+`CONSUMERS` and small `QUEUE_CAP` could
plausibly show `workerpool-channel` making the more frequent trips to the
kernel of the two - the reverse of what the two APIs "look like" they
should do.

## Setup

Build and start both containers - same workload, two queue mechanisms:
```sh
docker compose -f compose.yml up -d --build
```
- `workerpool-channel` / `workerpool-cond` - `PRODUCERS=8` /
  `CONSUMERS=8` goroutines each, queue capacity `QUEUE_CAP=16`, printing
  `produced/sec`/`consumed/sec` once a second.
```sh
docker logs -f lab-go-workerpool-channel
docker logs -f lab-go-workerpool-cond
```

To compare at different `PRODUCERS`/`CONSUMERS`/`QUEUE_CAP` values (in
particular, shrinking `QUEUE_CAP` relative to producer/consumer count for
Prediction 3's contention sweep), edit the `environment:` blocks in
`compose.yml` and rebuild:
```sh
docker compose -f compose.yml up -d --build
```

For anything needing OS/runtime-level introspection beyond throughput -
mutex/block profiling, or a syscall-level trace via the shared analysis
container - see [tools/](../tools/README.md) for how to get a shell in
`analysis` and target a container's cgroup.

## Experiments

TDB

## Tear down

```sh
docker compose -f compose.yml down
```
