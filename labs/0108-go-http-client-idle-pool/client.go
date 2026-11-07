// The service under study: calls the backend at a fixed rate (open loop,
// so calls start whether or not earlier ones have finished) through one
// shared http.Client. Its Transport is Go's DefaultTransport with only
// MaxIdleConnsPerHost changed.
//
// Logs every 5s:
//
//	ok, err      calls that succeeded / failed
//	p50, p99     call latency
//	new          share of calls that got a newly dialed connection rather
//	             than a reused one (httptrace GotConn.Reused == false)
//	dials        TCP connections dialed in the interval, including dials
//	             that finished after their call had already been handed a
//	             returned connection
//	peak         most calls in flight at once during the interval
//
// Knobs (env):
//
//	TARGET                   base URL (default http://lab-go-idlepool-backend:8080)
//	RATE                     calls per second (default 300)
//	MAX_IDLE_CONNS_PER_HOST  Transport.MaxIdleConnsPerHost; 0 means Go's
//	                         default of 2 (default 0)
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"
	"os"
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
		target = "http://lab-go-idlepool-backend:8080"
	}
	rate := envInt("RATE", 300)
	maxIdle := envInt("MAX_IDLE_CONNS_PER_HOST", 0)

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = maxIdle
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	log.Printf("target=%s rate=%d/s MaxIdleConnsPerHost=%d", target, rate, maxIdle)

	var (
		mu      sync.Mutex
		lats    []time.Duration
		errs    int
		newConn int

		dials, inflight, peak atomic.Int64
	)

	// Dials are counted from ConnectDone, which fires for every dial the
	// Transport makes on behalf of a call, whether or not that call ends
	// up using the connection.
	trace := &httptrace.ClientTrace{
		ConnectDone: func(_, _ string, err error) {
			if err == nil {
				dials.Add(1)
			}
		},
	}

	call := func() {
		n := inflight.Add(1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		defer inflight.Add(-1)

		reused := true
		t := *trace
		t.GotConn = func(info httptrace.GotConnInfo) { reused = info.Reused }
		req, _ := http.NewRequest(http.MethodGet, target+"/", nil)
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &t))

		t0 := time.Now()
		err := do(client, req)
		d := time.Since(t0)

		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errs++
			log.Printf("call error: %v", err)
			return
		}
		lats = append(lats, d)
		if !reused {
			newConn++
		}
	}

	go func() {
		start := time.Now()
		for range time.Tick(5 * time.Second) {
			mu.Lock()
			l, e, nc := lats, errs, newConn
			lats, errs, newConn = nil, 0, 0
			mu.Unlock()
			report(time.Since(start), l, e, nc, dials.Swap(0), peak.Swap(inflight.Load()))
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
// connection is eligible to go back to the idle pool.
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

func report(elapsed time.Duration, lats []time.Duration, errs, newConn int, dials, peak int64) {
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	pct := func(p float64) time.Duration {
		if len(lats) == 0 {
			return 0
		}
		return lats[int(p*float64(len(lats)-1))].Round(100 * time.Microsecond)
	}
	share := 0.0
	if len(lats) > 0 {
		share = 100 * float64(newConn) / float64(len(lats))
	}
	fmt.Printf("t=%4.0fs ok=%5d err=%3d p50=%-7s p99=%-7s new=%5.1f%% dials=%5d peak=%d\n",
		elapsed.Seconds(), len(lats), errs, pct(0.5), pct(0.99), share, dials, peak)
}
