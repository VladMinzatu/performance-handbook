## Thread and goroutine counts, for both modes

However many timers are running, and regardless of how much goroutine churn `AfterFunc` generates underneath, the real OS thread count shouldn't grow the way `TICKERS` or goroutine count does.

Start from the lab's default setup:
```sh
docker compose -f compose.yml up -d --build
```

Baseline thread counts, one `TICKERS=1` timer each:
```sh
echo "ticker:";    docker exec lab-go-timer-ticker    sh -c 'grep Threads /proc/1/status'
ticker:
Threads:	6

echo "afterfunc:"; docker exec lab-go-timer-afterfunc sh -c 'grep Threads /proc/1/status'
afterfunc:
Threads:	4
```

If we hold `WORK_MS=0` here deliberately we have purely a question of whether `TICKERS` count costs OS threads. 

```sh
for T in 1 100 1000 5000; do
  for M in ticker afterfunc; do
    docker rm -f lab-go-timer-sweep >/dev/null 2>&1
    docker run -d --name lab-go-timer-sweep \
      -e MODE=${M} -e TICKERS=$T -e INTERVAL_MS=100 -e WORK_MS=0 -e REPORT_SEC=5 \
      0106-go-timers-tickers-timer-${M}:latest >/dev/null
    sleep 8
    THREADS=$(docker exec lab-go-timer-sweep sh -c 'grep Threads /proc/1/status')
    LAST=$(docker logs lab-go-timer-sweep | tail -1)
    echo "MODE=${M} TICKERS=$T: $THREADS | $LAST"
    docker rm -f lab-go-timer-sweep >/dev/null 2>&1
  done
done
```

which produces the output:
```sh
MODE=ticker TICKERS=1: Threads:	5 | received=50 (+50) expected=50 goroutines_started=0 live_goroutines=2
MODE=afterfunc TICKERS=1: Threads:	5 | received=48 (+48) expected=50 goroutines_started=48 live_goroutines=1

MODE=ticker TICKERS=100: Threads:	6 | received=4901 (+4901) expected=5000 goroutines_started=0 live_goroutines=101
MODE=afterfunc TICKERS=100: Threads:	8 | received=4800 (+4800) expected=5000 goroutines_started=4800 live_goroutines=1

MODE=ticker TICKERS=1000: Threads:	9 | received=49024 (+49024) expected=50000 goroutines_started=0 live_goroutines=1001
MODE=afterfunc TICKERS=1000: Threads:	9 | received=48000 (+48000) expected=50000 goroutines_started=48000 live_goroutines=1

MODE=ticker TICKERS=5000: Threads:	10 | received=245183 (+245183) expected=250000 goroutines_started=0 live_goroutines=5001
MODE=afterfunc TICKERS=5000: Threads:	10 | received=235000 (+235000) expected=250000 goroutines_started=235000 live_goroutines=1
```
It's not perfectly flat, threads do creep up from ~5 to ~9-10 - but that growth is bounded, not proportional to `TICKERS`: a 1000x increase (1→1000) still only costs a handful more threads, and 1000→5000 (5x more) barely moves it at all (9→10). Both modes converge to the same ceiling despite `TICKERS` spanning three orders of magnitude - consistent with thread count tracking something like available parallelism, not raw timer or goroutine count.

The `live_goroutines` column is the more visible contrast between the two models, even though thread cost ends up the same: ticker mode holds one permanently-parked goroutine per ticker (`live_goroutines` tracks
`TICKERS+1` exactly - 2, 101, 1001, 5001), while afterfunc's spawned goroutines are so short-lived they never accumulate at all (`live_goroutines=1` throughout, even at `TICKERS=5000`) - but the churn is there, just not accumulating and visible here.

To clean up the containers, run:
```sh
docker compose -f compose.yml down
```
