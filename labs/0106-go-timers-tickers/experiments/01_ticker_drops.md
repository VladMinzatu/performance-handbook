## A slow Ticker consumer drops ticks, and the deficit keeps growing

Tests Prediction 1: with the consumer slower than the interval, the number of ticks actually received should fall further and further behind the number that would have fired on a fixed wall-clock schedule - and the gap should keep growing, not stabilize.

Start from the lab's default setup (`INTERVAL_MS=100`, `WORK_MS=250` - work slower than the interval):
```sh
docker compose -f compose.yml up -d --build
```

After a while, we can look at the full log - watch how `received` compares to `expected` over time, and whether the gap between them grows, shrinks, or holds steady:
```sh
docker logs lab-go-timer-ticker
mode=ticker tickers=1 interval_ms=100 work_ms=250 GOMAXPROCS=8
received=20 (+20) expected=50 goroutines_started=0 live_goroutines=2
received=40 (+20) expected=100 goroutines_started=0 live_goroutines=2
received=59 (+19) expected=150 goroutines_started=0 live_goroutines=2
received=79 (+20) expected=200 goroutines_started=0 live_goroutines=2
received=99 (+20) expected=250 goroutines_started=0 live_goroutines=2
```
As expected, `received` climbs by a steady ~20 every report - which is exactly `5000ms / 250ms`, i.e. the consumer's own `WORK_MS`, not the ticker's 100ms interval, is what's actually pacing throughput. The deficit (`expected - received`) grows every single report and never closes: 30 → 60 → 91 → 121 → 151 - roughly +30 each time, unbounded, not settling into some steady lag.

What happens is, the `Ticker` fires on its own fixed wall-clock schedule regardless of whether anyone's listening, and its channel only ever holds one pending tick. Once the consumer is mid-`WORK_MS`, every tick that fires before it comes back to receive finds the channel's single slot already occupied (or nobody ready to take it) and is silently thrown away right there - not queued, not replayed later. With `WORK_MS=250` and `INTERVAL_MS=100`, roughly 1-2 ticks fire and vanish during every handling cycle; the consumer only ever sees whatever's left waiting the instant it returns, which is at most one.

**What goes on under the hood**: `Ticker.C` *is* buffered - capacity exactly 1 (`make(chan Time, 1)` under the hood). When a tick fires, the runtime doesn't do a normal blocking channel send the way your own code would with `ch <- v`; it does the equivalent of `select { case ch <- now: default: }` - a *non-blocking* send. That choice isn't about our goroutine specifically - it's forced by where this code runs: the send happens from inside the runtime's own timer-processing path (whatever thread happened to notice the timer was due), not from a goroutine that's free to just go to sleep waiting for a receiver. Blocking there isn't an option, so the documented policy is explicit: try once, and if it can't succeed immediately, drop it.

Thus, "succeed immediately" is purely a property of *the channel*, not of our goroutine's state: the runtime never inspects what our goroutine is doing. A channel send (blocking or not) only has two ways to succeed:
hand the value directly to a goroutine that's already parked in a receive on that channel right now, or, failing that, place it in a free buffer slot. While we're in `time.Sleep()`, we're simply not one of the (possibly zero) goroutines parked on `<-ticker.C` - and the buffer's one slot is still holding whatever the *previous* tick left there, since we haven't drained it yet. So the non-blocking send finds neither a waiting receiver nor free buffer space, and gives up instantly. No inspection of "is the consumer busy" happens anywhere - it falls out entirely from "is anyone receiving right now, and is there room."

### A consumer faster than the interval

First, stop the running containers:
```sh
docker kill lab-go-timer-afterfunc; docker rm lab-go-timer-afterfunc
lab-go-timer-afterfunc
lab-go-timer-afterfunc

docker kill lab-go-timer-ticker; docker rm lab-go-timer-ticker      
lab-go-timer-ticker
lab-go-timer-ticker
```
Now let's see the same ticker, but with `WORK_MS` *below* `INTERVAL_MS` this time - if the drop mechanism is really about work exceeding the interval (not some fixed, unavoidable loss), this should show `received` tracking `expected` closely, with no growing gap:
```sh
docker run -d --name lab-go-timer-ticker-fast \
  -e MODE=ticker -e TICKERS=1 -e INTERVAL_MS=100 -e WORK_MS=50 -e REPORT_SEC=5 \
  0106-go-timers-tickers-timer-ticker:latest
sleep 16
docker logs lab-go-timer-ticker-fast
docker rm -f lab-go-timer-ticker-fast
```
which produces the output:
```sh
mode=ticker tickers=1 interval_ms=100 work_ms=50 GOMAXPROCS=8
received=50 (+50) expected=50 goroutines_started=0 live_goroutines=2
received=100 (+50) expected=100 goroutines_started=0 live_goroutines=2
received=149 (+49) expected=150 goroutines_started=0 live_goroutines=2
```

Now `received` tracks `expected` almost exactly (off by at most 1, just startup/rounding noise) - no growing gap at all. Same ticker, same mechanism, only difference is the consumer gets back to the channel before the next tick is due, so nothing ever finds the slot occupied.
