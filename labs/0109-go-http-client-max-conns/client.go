// The service under study: calls the backend at a fixed rate (open loop,
// so calls start whether or not earlier ones have finished) through one
// shared http.Client. Its Transport is Go's DefaultTransport with
// MaxConnsPerHost and MaxIdleConnsPerHost set from the environment. The
// idle limit defaults high enough that connections are always reused, so
// the connection limit is the only thing that changes between runs.
//
// Logs every 5s:
//
//	ok, err      calls that succeeded / failed in the interval (the first
//	             error of the interval is printed separately)
//	p50, p99     latency of successful calls, measured around the whole
//	             call, so it includes any time spent waiting for a
//	             connection
//	inflight     calls in progress right now
//	waiting      calls in progress that don't have a connection yet
//	             (between httptrace GetConn and GotConn), i.e. the queue
//	goroutines   runtime.NumGoroutine()
//	dials        TCP connections dialed in the interval
//
// Also serves net/http/pprof on :6060, for goroutine dumps:
//
//	curl 'http://lab-go-maxconns-client:6060/debug/pprof/goroutine?debug=1'
//
// Knobs (env):
//
//	TARGET                   base URL (default http://lab-go-maxconns-backend:8080)
//	RATE                     calls per second (default 300)
//	MAX_CONNS_PER_HOST       Transport.MaxConnsPerHost; 0 means no limit (default 0)
//	MAX_IDLE_CONNS_PER_HOST  Transport.MaxIdleConnsPerHost (default 512)
//	TIMEOUT                  http.Client.Timeout (default 5s)
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"
	_ "net/http/pprof"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

func main() {
	target := os.Getenv("TARGET")
	if target == "" {
		target = "http://lab-go-maxconns-backend:8080"
	}
	rate := envInt("RATE", 300)
	maxConns := envInt("MAX_CONNS_PER_HOST", 0)
	maxIdle := envInt("MAX_IDLE_CONNS_PER_HOST", 512)
	timeout, err := time.ParseDuration(os.Getenv("TIMEOUT"))
	if err != nil {
		timeout = 5 * time.Second
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxConnsPerHost = maxConns
	tr.MaxIdleConnsPerHost = maxIdle
	if tr.MaxIdleConns < maxIdle {
		tr.MaxIdleConns = maxIdle
	}
	client := &http.Client{Transport: tr, Timeout: timeout}
	log.Printf("target=%s rate=%d/s MaxConnsPerHost=%d MaxIdleConnsPerHost=%d timeout=%s",
		target, rate, maxConns, maxIdle, timeout)

	// pprof on its own listener; it doesn't use the client's Transport.
	go func() { log.Fatal(http.ListenAndServe(":6060", nil)) }()

	var (
		mu       sync.Mutex
		lats     []time.Duration
		errs     int
		firstErr error

		dials, inflight, waiting atomic.Int64
	)

	trace := &httptrace.ClientTrace{
		ConnectDone: func(_, _ string, err error) {
			if err == nil {
				dials.Add(1)
			}
		},
	}

	call := func() {
		inflight.Add(1)
		defer inflight.Add(-1)

		// waiting counts calls between asking the Transport for a
		// connection and getting one. A call that times out while still
		// waiting never reaches GotConn, so it's decremented at the end.
		var gotConn atomic.Bool
		t := *trace
		t.GetConn = func(string) { waiting.Add(1) }
		t.GotConn = func(httptrace.GotConnInfo) {
			if gotConn.CompareAndSwap(false, true) {
				waiting.Add(-1)
			}
		}
		defer func() {
			if gotConn.CompareAndSwap(false, true) {
				waiting.Add(-1)
			}
		}()

		req, _ := http.NewRequest(http.MethodGet, target+"/", nil)
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &t))

		t0 := time.Now()
		err := do(client, req)
		d := time.Since(t0)

		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errs++
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		lats = append(lats, d)
	}

	go func() {
		start := time.Now()
		for range time.Tick(5 * time.Second) {
			mu.Lock()
			l, e, fe := lats, errs, firstErr
			lats, errs, firstErr = nil, 0, nil
			mu.Unlock()
			report(time.Since(start), l, e, inflight.Load(), waiting.Load(), dials.Swap(0))
			if fe != nil {
				log.Printf("first error in interval: %v", fe)
			}
		}
	}()

	// Open loop: every 10ms, start however many calls are due by now.
	start := time.Now()
	sent := 0
	for range time.Tick(10 * time.Millisecond) {
		for due := int(time.Since(start).Seconds()*float64(rate)) - sent; due > 0; due-- {
			sent++
			go call()
		}
	}
}

// do makes one call and reads the body to EOF before closing it, so the
// connection goes back to the pool.
func do(client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func report(elapsed time.Duration, lats []time.Duration, errs int, inflight, waiting, dials int64) {
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	pct := func(p float64) time.Duration {
		if len(lats) == 0 {
			return 0
		}
		return lats[int(p*float64(len(lats)-1))].Round(100 * time.Microsecond)
	}
	fmt.Printf("t=%4.0fs ok=%5d err=%5d p50=%-8s p99=%-8s inflight=%5d waiting=%5d goroutines=%6d dials=%4d\n",
		elapsed.Seconds(), len(lats), errs, pct(0.5), pct(0.99),
		inflight, waiting, runtime.NumGoroutine(), dials)
}
