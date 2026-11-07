// The downstream service: GET / sleeps for a random service time between
// SERVICE_MIN_MS and SERVICE_MAX_MS, then returns a small JSON body. The
// random spread makes the number of calls in flight at the client go up
// and down, the way real traffic does.
//
// Logs every 5s: requests served, connections accepted in the interval,
// and connections currently open.
//
// Knobs (env):
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
	"strconv"
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
	minMS := envInt("SERVICE_MIN_MS", 10)
	maxMS := envInt("SERVICE_MAX_MS", 40)
	log.Printf("service time %d-%dms", minMS, maxMS)

	var requests, accepted, open atomic.Int64

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ms := minMS + rand.IntN(maxMS-minMS+1)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"service_ms":%d}`+"\n", ms)
		requests.Add(1)
	})

	srv := &http.Server{
		Addr: ":8080",
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
			fmt.Printf("t=%4.0fs requests=%5d accepted=%5d open=%d\n",
				time.Since(start).Seconds(), requests.Swap(0), accepted.Swap(0), open.Load())
		}
	}()

	log.Fatal(srv.ListenAndServe())
}
