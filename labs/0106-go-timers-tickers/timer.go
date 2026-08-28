// Timer/ticker workload for the timers-and-tickers lab. Spawns TICKERS
// independent, periodic firings every INTERVAL_MS, each doing WORK_MS of
// simulated work per firing - via one of two APIs:
//
//   - MODE=ticker:    a persistent goroutine per ticker, consuming from a
//     single time.Ticker's channel in a for-range loop. Ticker fires on
//     a fixed wall-clock cadence regardless of whether the consumer is
//     ready; if the consumer is still busy with the *previous* firing
//     when the next tick would fire, that tick is silently dropped (the
//     channel is buffered to depth 1, and a full/no-receiver send is a
//     non-blocking no-op) - it is never queued or made up for later.
//   - MODE=afterfunc: a self-rescheduling chain of time.AfterFunc calls.
//     Each firing runs in a *newly spawned* goroutine (documented
//     AfterFunc behavior), and only schedules the *next* firing once the
//     current one's work is done - so its cadence is actually
//     work+interval apart when work is slow, not a fixed external clock.
//     Nothing is ever silently dropped this way, but nothing is ever
//     "on schedule" either once work exceeds interval.
//
// Both modes report, once per REPORT_SEC: how many firings were actually
// processed, how many *would* have fired by now on a fixed wall-clock
// schedule (received/expected - the same formula for both modes, so the
// comparison is apples-to-apples even though what "expected" means
// differs subtly per mode), how many goroutines this process has
// self-reported starting to do work (only ever nonzero for
// MODE=afterfunc - MODE=ticker's work always runs on the same
// persistent goroutine), and the live goroutine count.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
)

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func main() {
	mode := os.Getenv("MODE")
	if mode == "" {
		mode = "ticker"
	}
	tickers := envInt("TICKERS", 1)
	intervalMS := envInt("INTERVAL_MS", 100)
	workMS := envInt("WORK_MS", 250)
	reportSec := envInt("REPORT_SEC", 5)

	interval := time.Duration(intervalMS) * time.Millisecond
	work := time.Duration(workMS) * time.Millisecond

	fmt.Printf("mode=%s tickers=%d interval_ms=%d work_ms=%d GOMAXPROCS=%d\n",
		mode, tickers, intervalMS, workMS, runtime.GOMAXPROCS(0))

	var received int64
	var goroutinesStarted int64

	start := time.Now()

	switch mode {
	case "afterfunc":
		for i := 0; i < tickers; i++ {
			var fn func()
			fn = func() {
				atomic.AddInt64(&goroutinesStarted, 1)
				atomic.AddInt64(&received, 1)
				if work > 0 {
					time.Sleep(work)
				}
				time.AfterFunc(interval, fn)
			}
			time.AfterFunc(interval, fn)
		}
	default:
		for i := 0; i < tickers; i++ {
			go func() {
				t := time.NewTicker(interval)
				defer t.Stop()
				for range t.C {
					atomic.AddInt64(&received, 1)
					if work > 0 {
						time.Sleep(work)
					}
				}
			}()
		}
	}

	report := time.NewTicker(time.Duration(reportSec) * time.Second)
	defer report.Stop()
	var lastReceived int64
	for range report.C {
		cur := atomic.LoadInt64(&received)
		expected := int64(time.Since(start)/interval) * int64(tickers)
		gs := atomic.LoadInt64(&goroutinesStarted)
		fmt.Printf("received=%d (+%d) expected=%d goroutines_started=%d live_goroutines=%d\n",
			cur, cur-lastReceived, expected, gs, runtime.NumGoroutine())
		lastReceived = cur
	}
}
