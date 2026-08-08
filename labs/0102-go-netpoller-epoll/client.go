// Connection-holding client and burst-wake controller for the netpoller
// lab. Opens CONNS concurrent TCP connections to the server and holds them
// idle - each one blocked on Read on the server side - until told, via the
// /burst HTTP endpoint, to write to every one of them at once and time how
// long the round trip takes. That moment is the interesting one: every
// parked goroutine on the server becomes runnable simultaneously and has
// to compete for GOMAXPROCS to actually run.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

func main() {
	serverAddr := os.Getenv("SERVER_ADDR")
	if serverAddr == "" {
		serverAddr = "server:9000"
	}
	numConns := 2000
	if v := os.Getenv("CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			numConns = n
		}
	}

	var mu sync.Mutex
	conns := make([]net.Conn, 0, numConns)

	// Dial in small batches rather than all at once - avoids a thundering
	// herd on the accept backlog and ephemeral port exhaustion when CONNS
	// is large.
	const batchSize = 200
	for start := 0; start < numConns; start += batchSize {
		end := start + batchSize
		if end > numConns {
			end = numConns
		}
		var wg sync.WaitGroup
		for i := start; i < end; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				conn, err := net.Dial("tcp", serverAddr)
				if err != nil {
					fmt.Println("dial error:", err)
					return
				}
				mu.Lock()
				conns = append(conns, conn)
				mu.Unlock()
			}()
		}
		wg.Wait()
	}
	fmt.Printf("holding %d connections to %s\n", len(conns), serverAddr)

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			n := len(conns)
			mu.Unlock()
			fmt.Printf("held_conns=%d\n", n)
		}
	}()

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n := len(conns)
		mu.Unlock()
		fmt.Fprintf(w, "%d\n", n)
	})

	http.HandleFunc("/burst", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		snapshot := make([]net.Conn, len(conns))
		copy(snapshot, conns)
		mu.Unlock()

		latencies := make([]time.Duration, len(snapshot))
		var wg sync.WaitGroup
		start := time.Now()
		for i, c := range snapshot {
			wg.Add(1)
			go func(i int, c net.Conn) {
				defer wg.Done()
				t0 := time.Now()
				buf := []byte{1}
				if _, err := c.Write(buf); err != nil {
					return
				}
				if _, err := c.Read(buf); err != nil {
					return
				}
				latencies[i] = time.Since(t0)
			}(i, c)
		}
		wg.Wait()
		total := time.Since(start)

		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		pct := func(p float64) time.Duration {
			if len(latencies) == 0 {
				return 0
			}
			idx := int(float64(len(latencies)-1) * p)
			return latencies[idx]
		}

		resp := map[string]string{
			"conns": strconv.Itoa(len(snapshot)),
			"total": total.String(),
			"p50":   pct(0.50).String(),
			"p99":   pct(0.99).String(),
			"max":   latencies[len(latencies)-1].String(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	fmt.Println("burst controller listening on :8090")
	if err := http.ListenAndServe(":8090", nil); err != nil {
		fmt.Println("http server error:", err)
		os.Exit(1)
	}
}
