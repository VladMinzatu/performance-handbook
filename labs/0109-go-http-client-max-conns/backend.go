// The downstream service: GET / sleeps for a random service time between
// a minimum and a maximum, then returns a small JSON body. The range can
// be changed while it runs, so a slowdown can be applied without
// restarting it or breaking the client's connections:
//
//	curl -X PUT 'http://lab-go-maxconns-backend:8080/latency?min=20&max=80'
//
// Logs every 5s: requests served, connections accepted in the interval,
// connections currently open, requests in progress, and its own p50/p99
// handling time (what the backend would report as its latency).
//
// Knobs (env), the starting range:
//
//	SERVICE_MIN_MS  shortest service time (default 10)
//	SERVICE_MAX_MS  longest service time (default 40)
package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
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
	var minMS, maxMS atomic.Int64
	minMS.Store(int64(envInt("SERVICE_MIN_MS", 10)))
	maxMS.Store(int64(envInt("SERVICE_MAX_MS", 40)))
	log.Printf("service time %d-%dms", minMS.Load(), maxMS.Load())

	var (
		accepted, open, active atomic.Int64

		mu   sync.Mutex
		lats []time.Duration
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		active.Add(1)
		defer active.Add(-1)
		t0 := time.Now()
		lo, hi := minMS.Load(), maxMS.Load()
		ms := lo + rand.Int64N(hi-lo+1)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"service_ms":%d}`+"\n", ms)
		d := time.Since(t0)
		mu.Lock()
		lats = append(lats, d)
		mu.Unlock()
	})
	mux.HandleFunc("PUT /latency", func(w http.ResponseWriter, r *http.Request) {
		lo, err1 := strconv.Atoi(r.URL.Query().Get("min"))
		hi, err2 := strconv.Atoi(r.URL.Query().Get("max"))
		if err1 != nil || err2 != nil || lo < 0 || hi < lo {
			http.Error(w, "usage: PUT /latency?min=<ms>&max=<ms>", http.StatusBadRequest)
			return
		}
		minMS.Store(int64(lo))
		maxMS.Store(int64(hi))
		log.Printf("service time now %d-%dms", lo, hi)
		fmt.Fprintf(w, "service time %d-%dms\n", lo, hi)
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
		ConnState: func(_ net.Conn, s http.ConnState) {
			switch s {
			case http.StateNew:
				accepted.Add(1)
				open.Add(1)
			case http.StateClosed, http.StateHijacked:
				open.Add(-1)
			}
		},
	}

	go func() {
		start := time.Now()
		for range time.Tick(5 * time.Second) {
			mu.Lock()
			l := lats
			lats = nil
			mu.Unlock()
			sort.Slice(l, func(i, j int) bool { return l[i] < l[j] })
			pct := func(p float64) time.Duration {
				if len(l) == 0 {
					return 0
				}
				return l[int(p*float64(len(l)-1))].Round(100 * time.Microsecond)
			}
			fmt.Printf("t=%4.0fs requests=%5d accepted=%4d open=%4d active=%4d p50=%-7s p99=%s\n",
				time.Since(start).Seconds(), len(l), accepted.Swap(0), open.Load(), active.Load(),
				pct(0.5), pct(0.99))
		}
	}()

	log.Fatal(srv.ListenAndServe())
}
