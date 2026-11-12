// The Go server under study, on net/http.
//
//	GET /cheap  returns a short response at once
//	GET /heavy  runs a fixed amount of arithmetic inline in the handler,
//	            then responds
//
// The /heavy loop count is calibrated at startup to take HEAVY_MS on an
// idle core. The tokio server calibrates the same loop the same way, so
// both servers do the same amount of work per request. The loop is a
// tight loop with no function calls, so the only thing that can stop it
// is the runtime's asynchronous preemption.
//
// Knobs (env):
//
//	HEAVY_MS    target duration of one /heavy call on an idle core (default 200)
//	GOMAXPROCS  number of Ps, i.e. worker threads (set from WORKERS in compose.yml)
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"
)

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

// spin is xorshift64 followed by a multiply, n times. The result is
// returned and written to the response so the loop can't be removed.
func spin(n uint64) uint64 {
	x := uint64(88172645463325252)
	for i := uint64(0); i < n; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		x *= 0x2545F4914F6CDD1D
	}
	return x
}

// sink keeps calibrate's results live; without it the compiler inlines
// spin and drops the unused arithmetic, leaving an empty loop.
var sink uint64

// calibrate returns the loop count that takes about target on this core,
// using the fastest of a few timed runs.
func calibrate(target time.Duration) uint64 {
	const probe = 20_000_000
	best := time.Duration(1<<63 - 1)
	for i := 0; i < 5; i++ {
		t0 := time.Now()
		sink += spin(probe)
		if d := time.Since(t0); d < best {
			best = d
		}
	}
	return uint64(float64(probe) * float64(target) / float64(best))
}

func main() {
	heavy := time.Duration(envInt("HEAVY_MS", 200)) * time.Millisecond
	iters := calibrate(heavy)
	log.Printf("GOMAXPROCS=%d HEAVY_MS=%d iters=%d", runtime.GOMAXPROCS(0), heavy.Milliseconds(), iters)

	http.HandleFunc("/cheap", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	http.HandleFunc("/heavy", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, spin(iters))
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
