// The load generator. Sends two kinds of traffic to one server:
//
//   - /cheap at RATE requests per second, open loop: requests start on
//     schedule whether or not earlier ones have finished, so a stalled
//     server shows up as latency rather than as a lower request rate.
//   - /heavy with HEAVY_CONC requests always in flight, closed loop: each
//     finished request is replaced right away.
//
// Both go through one shared http.Client with an idle pool large enough
// that connections are never re-dialed.
//
// Logs every 5s:
//
//	cheap ok, err      /cheap requests that succeeded / failed
//	p50, p99, max      /cheap latency
//	heavy ok, mean     /heavy requests completed and their mean latency
//
// Knobs (env):
//
//	TARGET      go or tokio, or a base URL (default go)
//	RATE        /cheap requests per second (default 200)
//	HEAVY_CONC  /heavy requests kept in flight (default 2)
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
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
	switch target {
	case "", "go":
		target = "http://lab-go-preempt-goserver:8080"
	case "tokio":
		target = "http://lab-go-preempt-tokioserver:8080"
	}
	rate := envInt("RATE", 200)
	heavyConc := envInt("HEAVY_CONC", 2)

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConns = 1024
	tr.MaxIdleConnsPerHost = 1024
	client := &http.Client{Transport: tr, Timeout: 10 * time.Second}
	log.Printf("target=%s rate=%d/s heavy_conc=%d", target, rate, heavyConc)

	// The servers calibrate their loop before listening, so wait until
	// the target answers rather than logging startup errors as failures.
	for do(client, target+"/cheap") != nil {
		time.Sleep(200 * time.Millisecond)
	}

	var (
		mu        sync.Mutex
		cheapLats []time.Duration
		cheapErrs int
		heavyLats []time.Duration
	)

	get := func(path string) (time.Duration, error) {
		t0 := time.Now()
		err := do(client, target+path)
		return time.Since(t0), err
	}

	cheap := func() {
		d, err := get("/cheap")
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			cheapErrs++
			log.Printf("cheap error: %v", err)
			return
		}
		cheapLats = append(cheapLats, d)
	}

	for i := 0; i < heavyConc; i++ {
		go func() {
			for {
				d, err := get("/heavy")
				if err != nil {
					log.Printf("heavy error: %v", err)
					time.Sleep(time.Second)
					continue
				}
				mu.Lock()
				heavyLats = append(heavyLats, d)
				mu.Unlock()
			}
		}()
	}

	go func() {
		start := time.Now()
		for range time.Tick(5 * time.Second) {
			mu.Lock()
			c, e, h := cheapLats, cheapErrs, heavyLats
			cheapLats, cheapErrs, heavyLats = nil, 0, nil
			mu.Unlock()
			report(time.Since(start), c, e, h)
		}
	}()

	// Open loop: every 5ms, start however many /cheap requests are due.
	start := time.Now()
	sent := 0
	for range time.Tick(5 * time.Millisecond) {
		for due := int(time.Since(start).Seconds()*float64(rate)) - sent; due > 0; due-- {
			sent++
			go cheap()
		}
	}
}

// do makes one request and reads the body to EOF, so the connection goes
// back to the idle pool.
func do(client *http.Client, url string) error {
	resp, err := client.Get(url)
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

func report(elapsed time.Duration, cheap []time.Duration, errs int, heavy []time.Duration) {
	sort.Slice(cheap, func(i, j int) bool { return cheap[i] < cheap[j] })
	pct := func(p float64) time.Duration {
		if len(cheap) == 0 {
			return 0
		}
		return cheap[int(p*float64(len(cheap)-1))].Round(100 * time.Microsecond)
	}
	var sum time.Duration
	for _, d := range heavy {
		sum += d
	}
	mean := time.Duration(0)
	if len(heavy) > 0 {
		mean = (sum / time.Duration(len(heavy))).Round(time.Millisecond)
	}
	fmt.Printf("t=%4.0fs cheap ok=%5d err=%3d p50=%-8s p99=%-8s max=%-8s | heavy ok=%3d mean=%s\n",
		elapsed.Seconds(), len(cheap), errs, pct(0.5), pct(0.99), pct(1), len(heavy), mean)
}
