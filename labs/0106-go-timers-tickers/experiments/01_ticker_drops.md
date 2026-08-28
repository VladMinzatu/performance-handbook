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
