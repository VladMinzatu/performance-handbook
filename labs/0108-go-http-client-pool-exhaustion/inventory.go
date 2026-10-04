// Downstream service for the client pool-exhaustion lab. Every request
// costs SERVICE_MS of (sleeping) work, and at most WORKERS requests are
// worked on at once - the rest wait for a worker. The service time can be
// changed at runtime to simulate the downstream getting slower without
// restarting anything.
//
// Reports its own view of the world once a second: request rate and the
// latency it measured itself, from the moment a request reached the
// handler to the moment it finished. This is the "downstream dashboard"
// that stays flat while the client's latency explodes.
package main

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil {
		return v
	}
	return def
}

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
	port := envInt("PORT", 8080)
	workers := envInt("WORKERS", 200)
	var serviceMs atomic.Int64
	serviceMs.Store(int64(envInt("SERVICE_MS", 50)))

	sem := make(chan struct{}, workers)
	var inFlight, queued atomic.Int64
	var lat window

	mux := http.NewServeMux()
	mux.HandleFunc("/item", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		inFlight.Add(1)
		defer inFlight.Add(-1)

		queued.Add(1)
		sem <- struct{}{}
		queued.Add(-1)
		time.Sleep(time.Duration(serviceMs.Load()) * time.Millisecond)
		<-sem

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"sku":"abc-123","stock":42}`)
		lat.add(time.Since(start))
	})
	mux.HandleFunc("/admin/service-time", func(w http.ResponseWriter, r *http.Request) {
		v, err := strconv.Atoi(r.URL.Query().Get("ms"))
		if err != nil || v < 0 {
			http.Error(w, "usage: /admin/service-time?ms=<int>", http.StatusBadRequest)
			return
		}
		serviceMs.Store(int64(v))
		fmt.Fprintf(w, "service time set to %dms\n", v)
	})

	go func() {
		for range time.Tick(time.Second) {
			s := lat.drain()
			fmt.Printf("inventory rps=%-5d p50=%-8s p99=%-8s in_flight=%-5d queued=%-5d service_ms=%d workers=%d\n",
				len(s), ms(pct(s, 0.50)), ms(pct(s, 0.99)),
				inFlight.Load(), queued.Load(), serviceMs.Load(), workers)
		}
	}()

	fmt.Printf("inventory listening on :%d (SERVICE_MS=%d WORKERS=%d)\n", port, serviceMs.Load(), workers)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
