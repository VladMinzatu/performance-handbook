# Timers and tickers: what happens when a tick is missed

Uses the shared lab infrastructure in [tools/](../tools/README.md), but
only for the `analysis` container's tracing/profiling, if the
experiments need it - none of this lab's containers need `labnet` or any
network reachability at all.

## Background

`time.Timer` and `time.Ticker` don't spawn a goroutine or an OS thread
just by existing. Creating one registers an entry in the Go runtime's
own internal timer bookkeeping (a per-P min-heap, checked opportunistically
whenever a P looks for work, plus a background monitor that can wake a
P specifically because a timer is due); when it fires, the runtime does
a **non-blocking** send of the current time on the timer's channel.
`Ticker.C` is buffered to depth exactly one - so if nothing has received
the last tick by the time the next one is due, that send simply fails
and the tick is gone. Not queued, not replayed, no error - it just never
happened as far as the receiver can tell.

`time.AfterFunc` is a different shape entirely: instead of a channel a
consumer has to actively receive from, it runs a callback function *for*
you - in a **new goroutine, spawned fresh on every firing** (this is
documented behavior, not an implementation detail). And instead of
firing on an independent, fixed wall-clock schedule, a self-rescheduling
`AfterFunc` chain (each firing calls `AfterFunc` again for the next one)
only starts counting down the next interval once the *current* callback
returns - so a slow callback doesn't cause dropped firings, it just
stretches the effective period out.

Both are usually reached for interchangeably for "do this repeatedly,"
but they fail differently under exactly the situation this lab is about:
a handler that occasionally (or always) takes longer than the interval
between firings.

## Hypotheses

**Prediction 1 - a slow consumer causes a `Ticker` to silently drop
ticks, not queue them or catch up.** With `WORK_MS` (simulated handling
time per tick) set higher than `INTERVAL_MS`, the number of ticks
actually received should fall further and further behind the number
that *would* have fired on a fixed wall-clock schedule - and the gap
should keep growing over time, not stabilize, since every tick that
fires while the consumer is still busy is simply gone for good.

**Prediction 2 - `AfterFunc` never drops a firing, but its cadence
stretches to `work + interval` instead of staying at `interval`.**
Because a self-rescheduling `AfterFunc` chain only schedules the next
firing after the current callback finishes, it should process every
single firing it schedules (no silent gaps the way `Ticker` has) - but
its actual throughput (firings per second) should end up *lower* than
`Ticker`'s under the same `WORK_MS`/`INTERVAL_MS`, since it never
overlaps a firing with the next one's wait time the way a fast-draining
`Ticker` consumer effectively can.

**Prediction 3 - none of this costs anything at the OS thread level,
however many timers are running.** Whether it's a handful of tickers or
thousands, and regardless of how much goroutine churn `AfterFunc` is
generating underneath, the process's real OS thread count should stay
flat - timer bookkeeping and goroutine creation/exit are both handled
entirely within the Go runtime, never touching the kernel just because
a timer fired or a new goroutine was spawned to run a callback.

## Setup

Build and start both containers - identical image, identical `TICKERS`/
`INTERVAL_MS`/`WORK_MS`, differing only in `MODE`:
```sh
docker compose -f compose.yml up -d --build
```
Both default to `WORK_MS=250` against `INTERVAL_MS=100` - work
deliberately slower than the interval, so the interesting behavior is
visible immediately. Each prints `received` (firings actually
processed), `expected` (what a fixed wall-clock schedule would have
produced by now), `goroutines_started` (self-tracked; only ever nonzero
for `MODE=afterfunc`), and the live goroutine count, once every
`REPORT_SEC`:
```sh
docker logs -f lab-go-timer-ticker
docker logs -f lab-go-timer-afterfunc
```

To compare at different `TICKERS`/`INTERVAL_MS`/`WORK_MS` values, edit
the `environment:` blocks in `compose.yml` and rebuild, or override for
a one-off run:
```sh
docker run -d --name lab-go-timer-ticker \
  -e MODE=ticker -e TICKERS=1000 -e INTERVAL_MS=100 -e WORK_MS=0 \
  0106-go-timers-tickers-timer-ticker:latest
```

Real OS thread counts, for Prediction 3:
```sh
docker exec lab-go-timer-ticker    sh -c 'grep Threads /proc/1/status'
docker exec lab-go-timer-afterfunc sh -c 'grep Threads /proc/1/status'
```

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down
```
