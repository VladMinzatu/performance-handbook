// Open-loop load generator for the client pool-exhaustion lab. Requests
// are sent on a fixed schedule (RATE per second) whether or not earlier
// ones have completed - the way independent users behave. A closed-loop
// generator (N workers each waiting for their previous response) would
// slow down along with the system under test and hide the queue building
// up in front of it.
//
// Reports once a second: requests sent, completed, errors, in-flight, and
// latency of the requests that completed in that second. Prints a summary
// over the whole run at the end.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// window collects latency samples for one reporting interval.
type window struct {
	mu      sync.Mutex
	samples []time.Duration
}

func (w *window) add(d time.Duration) {
	w.mu.Lock()
	w.samples = append(w.samples, d)
	w.mu.Unlock()
}

func (w *window) drain() []time.Duration {
	w.mu.Lock()
	s := w.samples
	w.samples = nil
	w.mu.Unlock()
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s
}

func pct(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[int(float64(len(sorted)-1)*p)]
}

func ms(d time.Duration) string {
	return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
}

func main() {
	url := flag.String("url", "http://edge:8080/checkout", "target URL")
	rate := flag.Int("rate", 100, "requests per second")
	duration := flag.Duration("duration", 30*time.Second, "how long to send requests for")
	drain := flag.Duration("drain", 60*time.Second, "how long to wait for in-flight requests after sending stops")
	flag.Parse()

	// Unlimited connections, and enough idle ones kept that the generator
	// itself never becomes a pool bottleneck or churns connections.
	client := &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost:     0,
			MaxIdleConns:        0,
			MaxIdleConnsPerHost: 100000,
		},
	}

	var sent, completed, errors, inFlight atomic.Int64
	var lat window
	var all []time.Duration
	var allMu sync.Mutex
	var wg sync.WaitGroup

	fire := func() {
		defer wg.Done()
		defer inFlight.Add(-1)
		start := time.Now()
		resp, err := client.Get(*url)
		if err != nil {
			errors.Add(1)
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			errors.Add(1)
			return
		}
		d := time.Since(start)
		completed.Add(1)
		lat.add(d)
		allMu.Lock()
		all = append(all, d)
		allMu.Unlock()
	}

	report := func() {
		s := lat.drain()
		fmt.Printf("loadgen sent=%-5d completed=%-5d errors=%-4d in_flight=%-6d p50=%-9s p99=%-9s max=%s\n",
			sent.Swap(0), completed.Swap(0), errors.Swap(0), inFlight.Load(),
			ms(pct(s, 0.50)), ms(pct(s, 0.99)), ms(pct(s, 1)))
	}

	fmt.Printf("loadgen: %d req/s for %s against %s (open loop)\n", *rate, *duration, *url)
	stopReport := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				report()
			case <-stopReport:
				return
			}
		}
	}()

	// Schedule each request at start + i*interval, so a slow iteration
	// doesn't lower the offered rate - missed send times are caught up.
	interval := time.Second / time.Duration(*rate)
	start := time.Now()
	total := int(duration.Seconds() * float64(*rate))
	for i := 0; i < total; i++ {
		if d := time.Until(start.Add(time.Duration(i) * interval)); d > 0 {
			time.Sleep(d)
		}
		sent.Add(1)
		inFlight.Add(1)
		wg.Add(1)
		go fire()
	}

	fmt.Printf("loadgen: sending stopped, waiting up to %s for %d in-flight requests\n", *drain, inFlight.Load())
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(*drain):
		fmt.Printf("loadgen: drain timed out with %d requests still in flight\n", inFlight.Load())
	}
	close(stopReport)
	report()

	allMu.Lock()
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	fmt.Printf("loadgen summary: completed=%d p50=%s p90=%s p99=%s max=%s\n",
		len(all), ms(pct(all, 0.50)), ms(pct(all, 0.90)), ms(pct(all, 0.99)), ms(pct(all, 1)))
	allMu.Unlock()
	os.Exit(0)
}
