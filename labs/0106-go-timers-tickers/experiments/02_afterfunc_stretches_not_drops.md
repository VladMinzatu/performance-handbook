## AfterFunc never drops a firing, but its cadence stretches to work+interval

Tests Prediction 2: a self-rescheduling `AfterFunc` chain should process every single firing it schedules (no silent gaps the way `Ticker` has), but its actual cadence should settle at `WORK_MS + INTERVAL_MS`, not
stay at `INTERVAL_MS` - since it only schedules the *next* firing once the *current* one's work is done.

Start from the lab's default setup (`INTERVAL_MS=100`, `WORK_MS=250`):
```sh
docker compose -f compose.yml up -d --build
```

Let it run for a while, then look at the full log - two things to check: whether `received` and `goroutines_started` track each other exactly (confirming nothing is silently skipped, and every firing really does run in its own goroutine), and what rate `received` climbs at:
```sh
docker logs lab-go-timer-afterfunc
mode=afterfunc tickers=1 interval_ms=100 work_ms=250 GOMAXPROCS=8
received=14 (+14) expected=50 goroutines_started=14 live_goroutines=1
received=28 (+14) expected=100 goroutines_started=28 live_goroutines=1
received=42 (+14) expected=150 goroutines_started=42 live_goroutines=1
received=56 (+14) expected=200 goroutines_started=56 live_goroutines=1
```

To check for different configurations of `WORK_MS` (keeping `INTERVAL_MS=100` fixed):
```sh
for W in 50 100 500; do
  docker rm -f lab-go-timer-afterfunc-sweep >/dev/null 2>&1
  docker run -d --name lab-go-timer-afterfunc-sweep \
    -e MODE=afterfunc -e TICKERS=1 -e INTERVAL_MS=100 -e WORK_MS=$W -e REPORT_SEC=5 \
    0106-go-timers-tickers-timer-afterfunc:latest >/dev/null
  sleep 11
  echo "=== WORK_MS=$W ==="
  docker logs lab-go-timer-afterfunc-sweep
  docker rm -f lab-go-timer-afterfunc-sweep >/dev/null 2>&1
done
```
which produces the output:
```sh
=== WORK_MS=50 ===
mode=afterfunc tickers=1 interval_ms=100 work_ms=50 GOMAXPROCS=8
received=32 (+32) expected=50 goroutines_started=32 live_goroutines=1
received=64 (+32) expected=100 goroutines_started=64 live_goroutines=1

=== WORK_MS=100 ===
mode=afterfunc tickers=1 interval_ms=100 work_ms=100 GOMAXPROCS=8
received=24 (+24) expected=50 goroutines_started=24 live_goroutines=1
received=48 (+24) expected=100 goroutines_started=48 live_goroutines=2

=== WORK_MS=500 ===
mode=afterfunc tickers=1 interval_ms=100 work_ms=500 GOMAXPROCS=8
received=9 (+9) expected=50 goroutines_started=9 live_goroutines=2
received=17 (+8) expected=100 goroutines_started=17 live_goroutines=2
```

To clean up the containers, run:
```sh
docker compose -f compose.yml down
```
