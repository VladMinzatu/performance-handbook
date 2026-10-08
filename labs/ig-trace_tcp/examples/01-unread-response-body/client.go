// A service that checks whether orders exist by calling the orders API at a
// fixed rate (open loop) through one shared http.Client, the way most Go
// services do. It only needs the status code, never the body - the same
// shape as health checkers, webhook senders and "does X exist" lookups.
// Logs every 5s: requests, errors, latency, goroutines, open fds.
//
// Knobs (env):
//
//	TARGET  base URL (default http://tcp01-server:8080)
//	RATE    requests per second (default 200)
//	BODY    what happens to the unread response body:
//	          leak  - nothing: never read, never closed. The connection
//	                  stays tied to the body until the client's 5s timeout
//	                  tears it down, so every request opens a new one
//	          close - closed without reading: the transport can't reuse a
//	                  connection with unread bytes on it, so it closes it
//	                  and every request opens a new one
//	          drain - read to EOF, then closed: the connection goes back
//	                  to the idle pool and is reused (default leak)
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	target := env("TARGET", "http://tcp01-server:8080")
	rate, _ := strconv.Atoi(env("RATE", "200"))
	body := env("BODY", "leak")
	log.Printf("target=%s rate=%d/s body=%s", target, rate, body)

	client := &http.Client{Timeout: 5 * time.Second}

	var (
		mu    sync.Mutex
		lats  []time.Duration
		errs  int
		start = time.Now()
	)

	call := func(n int) {
		t0 := time.Now()
		err := checkOrder(client, fmt.Sprintf("%s/orders/%d", target, n), body)
		d := time.Since(t0)
		mu.Lock()
		if err != nil {
			errs++
		} else {
			lats = append(lats, d)
		}
		mu.Unlock()
	}

	go func() {
		for range time.Tick(5 * time.Second) {
			mu.Lock()
			l, e := lats, errs
			lats, errs = nil, 0
			mu.Unlock()
			report(time.Since(start), l, e)
		}
	}()

	// Open loop: every 10ms, start however many requests are due by now.
	sent := 0
	for range time.Tick(10 * time.Millisecond) {
		for due := int(time.Since(start).Seconds()*float64(rate)) - sent; due > 0; due-- {
			sent++
			go call(sent)
		}
	}
}

func checkOrder(client *http.Client, url, body string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	switch body {
	case "close":
		resp.Body.Close()
	case "drain":
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func openFDs() int {
	fds, _ := os.ReadDir("/proc/self/fd")
	return len(fds)
}

func report(elapsed time.Duration, lats []time.Duration, errs int) {
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	pct := func(p float64) time.Duration {
		if len(lats) == 0 {
			return 0
		}
		return lats[int(p*float64(len(lats)-1))].Round(10 * time.Microsecond)
	}
	fmt.Printf("t=%4.0fs ok=%5d err=%3d p50=%-8s p99=%-8s goroutines=%-6d fds=%d\n",
		elapsed.Seconds(), len(lats), errs, pct(0.5), pct(0.99),
		runtime.NumGoroutine(), openFDs())
}
