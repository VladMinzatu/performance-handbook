# Locking vs. channels: coordination overhead compared

Uses the shared lab infrastructure in [tools/](../tools/README.md), but
only for the `analysis` container's tracing/profiling, if the
experiments need it - none of this lab's containers need `labnet` or 
any network reachability at all.

## Background

Go's own proverb - "don't communicate by sharing memory; share memory by
communicating" - reads like channels should simply replace mutexes. They
don't, not on performance grounds anyway: a channel isn't a lock-free
alternative to `sync.Mutex`, it's a different data structure built
*using* one. A Go channel's internal state (its buffer, its queues of
waiting senders/receivers) is itself protected by a `runtime.mutex`
(`hchan.lock`) - so sending a value on a channel does real lock/unlock
work internally, plus bookkeeping a raw `sync.Mutex` never has to do:
managing a queue of waiting goroutines (`sudog` structs), and copying the
value across. Channels aren't "no lock"; they're "a lock, plus more,
in exchange for built-in blocking and hand-off semantics a bare mutex
doesn't have."

That extra machinery isn't free, but it isn't wasted either - it depends
entirely on what the coordination is actually *for*:

- **Protecting a piece of shared state in place** - a counter, a map, a
  cache entry - is exactly what a mutex is built for: lock, touch the
  state, unlock. Routing the same access through a channel to a single
  owner goroutine means paying the channel's extra machinery on top of
  what the mutex already provides, for no extra capability - the state
  still ends up touched by exactly one goroutine at a time either way.
- **Coordinating a hand-off** - "you go now, then you" - is exactly what
  a bare mutex *can't* express on its own. A `sync.Mutex` has no way to
  block a goroutine until a specific condition holds; that needs a
  `sync.Cond` layered on top, and `Cond.Broadcast` wakes *every* waiter
  on every signal (there's no way to target just the one goroutine whose
  turn it actually is), unlike a channel send, which the runtime can
  hand directly to the one goroutine waiting to receive it.

## Hypotheses

**Prediction 1 - for protecting simple shared state, a direct mutex
should beat a channel-mediated equivalent.** `counter-mutex` (`WORKERS`
goroutines lock, increment, unlock) and `counter-channel` (`WORKERS`
goroutines send a request over an unbuffered channel to a single owner
goroutine, which does the increment) do the exact same logical amount of
work at the same `WORKERS`. The channel version has to additionally pay
for `hchan.lock`, `sudog` queue management, and an extra goroutine hop
(request → owner → increment, instead of increment happening directly)
- so `counter-mutex`'s `ops/sec` should be meaningfully higher.

**Prediction 2 - for pure coordinated hand-off, channels should beat a
mutex-and-condvar equivalent.** `pingpong-channel` (a ring of unbuffered
channels passing a token) and `pingpong-mutex` (a shared "whose turn is
it" integer, guarded by a `sync.Mutex` and blocked on via `sync.Cond`)
both do nothing but pass a token around `PARTICIPANTS` goroutines,
forever. A channel send wakes exactly the one goroutine next in line;
`Cond.Broadcast` wakes *all* of them, and all but one immediately find it
isn't their turn and go back to waiting - a real, structural cost paid
once per hand-off that the channel ring doesn't have. So
`pingpong-channel`'s `handoffs/sec` should be substantially higher than
`pingpong-mutex`'s - the opposite direction from Prediction 1, same
underlying reason (channels do more, or less, useful work per unit of
machinery depending on the shape of the coordination).

**Prediction 3 - the difference shows up in CPU usage, not just
throughput.** `sync.Mutex` uses an adaptive spin before parking a
goroutine that can't immediately acquire a contended lock - real CPU
cycles burned on the chance the holder releases soon, potentially across
several cores spinning at once. Channel operations that can't proceed
have no equivalent spin phase in their slow path; they park directly.
So `docker stats`' CPU% for `counter-mutex` should be higher than
`counter-channel`'s by more than the throughput difference alone would
explain - i.e. CPU cost *per completed operation* should be higher for
the mutex path, not just CPU cost in total.

**Prediction 4 (stretch) - the mutex-and-condvar ring's relative
disadvantage should grow with `PARTICIPANTS`, exposing the
`Broadcast` thundering herd directly.** Every hand-off in
`pingpong-mutex` wakes all `PARTICIPANTS` waiters to let one of them
proceed; every hand-off in `pingpong-channel` wakes exactly one,
regardless of ring size. Sweeping `PARTICIPANTS` up should degrade
`pingpong-mutex`'s `handoffs/sec` faster than `pingpong-channel`'s - the
gap between them widening with ring size, not staying constant - since
the wasted-wakeup cost scales with the number of goroutines that get
woken for nothing, which `Broadcast` ties directly to `PARTICIPANTS`.

## Setup

Build and start all four containers - two coordination patterns, each
run once per mechanism:
```sh
docker compose -f compose.yml up -d --build
```
- `counter-mutex` / `counter-channel` - shared-counter coordination,
  `WORKERS=32` goroutines each, printing `ops/sec` once a second.
- `pingpong-mutex` / `pingpong-channel` - token-ring hand-off,
  `PARTICIPANTS=4` goroutines each, printing `handoffs/sec` once a
  second.
```sh
docker logs -f lab-go-counter-mutex
docker logs -f lab-go-counter-channel
docker logs -f lab-go-pingpong-mutex
docker logs -f lab-go-pingpong-channel
```

To compare at different `WORKERS`/`PARTICIPANTS` values, edit the
`environment:` blocks in `compose.yml` and rebuild:
```sh
docker compose -f compose.yml up -d --build
```

For Prediction 3's CPU comparison, `docker stats` needs no extra setup:
```sh
docker stats --no-stream lab-go-counter-mutex lab-go-counter-channel lab-go-pingpong-mutex lab-go-pingpong-channel
```

For anything needing OS/runtime-level introspection beyond throughput
and CPU% - `pprof`'s mutex/block profiles, `GODEBUG=schedtrace`, or
`bpftrace` via the shared analysis container - the techniques (and their
rough edges) are already worked out in
[lab 0104's experiments](../0104-go-atomics-vs-mutex-contention/experiments);
reuse rather than rediscover them here if a prediction calls for that
level of detail.

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down
```
