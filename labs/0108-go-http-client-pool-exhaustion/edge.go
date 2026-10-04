// Client-facing service for the client pool-exhaustion lab. Each
// GET /checkout makes one call to inventory through a single shared
// http.Client whose Transport caps connections to inventory at
// MAX_CONNS (MaxConnsPerHost). Once that many are busy, further calls
// block inside the Transport until a connection frees up - no error, no
// packets, no CPU, just waiting.
//
// Reports once a second: completed rate, in-flight requests, end-to-end
// latency, and an httptrace split of each inventory call into
// conn_wait (asking the pool for a connection -> getting one) and
// ttfb (request written -> first response byte). The split is a
// cross-check for what the OS-level tools show from outside.
//
// pprof (including the goroutine dump) is served on PPROF_PORT.
package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	_ "net/http/pprof"
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

func envStr(name, def string) string {
	if v := os.Getenv(name); v != "" {
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
	pprofPort := envInt("PPROF_PORT", 6060)
	maxConns := envInt("MAX_CONNS", 20)
	inventoryURL := envStr("INVENTORY_URL", "http://inventory:8080/item")

	// MaxIdleConnsPerHost matches MaxConnsPerHost so connections returned
	// to the pool stay open - otherwise the default (2) closes most of
	// them between bursts and the pool churns instead of just filling up.
	// No client timeout: requests wait for a connection rather than fail.
	client := &http.Client{
		Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			MaxConnsPerHost:     maxConns,
			MaxIdleConns:        0,
			MaxIdleConnsPerHost: maxConns,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	var inFlight atomic.Int64
	var errors atomic.Int64
	var total, connWait, ttfb window

	mux := http.NewServeMux()
	mux.HandleFunc("/checkout", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		inFlight.Add(1)
		defer inFlight.Add(-1)

		var getConn, gotConn, wrote, firstByte time.Time
		trace := &httptrace.ClientTrace{
			GetConn:              func(string) { getConn = time.Now() },
			GotConn:              func(httptrace.GotConnInfo) { gotConn = time.Now() },
			WroteRequest:         func(httptrace.WroteRequestInfo) { wrote = time.Now() },
			GotFirstResponseByte: func() { firstByte = time.Now() },
		}
		req, _ := http.NewRequestWithContext(httptrace.WithClientTrace(r.Context(), trace), http.MethodGet, inventoryURL, nil)

		resp, err := client.Do(req)
		if err != nil {
			errors.Add(1)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok"}`)

		total.add(time.Since(start))
		connWait.add(gotConn.Sub(getConn))
		ttfb.add(firstByte.Sub(wrote))
	})

	go func() {
		for range time.Tick(time.Second) {
			t, cw, fb := total.drain(), connWait.drain(), ttfb.drain()
			fmt.Printf("edge rps=%-5d in_flight=%-5d errors=%-4d total p50=%-9s p99=%-9s | conn_wait p50=%-9s p99=%-9s | ttfb p50=%-8s p99=%-8s\n",
				len(t), inFlight.Load(), errors.Swap(0),
				ms(pct(t, 0.50)), ms(pct(t, 0.99)),
				ms(pct(cw, 0.50)), ms(pct(cw, 0.99)),
				ms(pct(fb, 0.50)), ms(pct(fb, 0.99)))
		}
	}()

	// net/http/pprof registers on DefaultServeMux.
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", pprofPort), nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	fmt.Printf("edge listening on :%d (MAX_CONNS=%d, pprof on :%d, inventory at %s)\n", port, maxConns, pprofPort, inventoryURL)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
