// Shared-state coordination workload for the locking-vs-channels lab.
// WORKERS goroutines all need the same counter incremented, as fast as
// possible, forever - via one of two coordination mechanisms:
//
//   - MECH=mutex:   each worker locks a shared sync.Mutex, increments the
//     counter itself, unlocks. Direct, symmetric access.
//   - MECH=channel: each worker sends a request on an *unbuffered*
//     channel to a single dedicated owner goroutine, which is the only
//     one that ever touches the counter. "Share memory by
//     communicating" instead of "communicate by sharing memory" -
//     ownership is transferred/requested rather than the state being
//     protected in place.
//
// The channel is deliberately unbuffered: a send only completes once the
// owner is ready to receive, the same synchronous, one-at-a-time
// exclusion a mutex provides - so the two mechanisms are being compared
// on equivalent terms, not "mutex" vs. "mutex plus free pipelining".
package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func main() {
	mech := os.Getenv("MECH")
	if mech == "" {
		mech = "mutex"
	}
	workers := envInt("WORKERS", 32)

	fmt.Printf("mech=%s workers=%d GOMAXPROCS=%d\n", mech, workers, runtime.GOMAXPROCS(0))

	var counter int64

	switch mech {
	case "channel":
		reqs := make(chan struct{})
		go func() {
			for range reqs {
				atomic.AddInt64(&counter, 1)
			}
		}()
		for i := 0; i < workers; i++ {
			go func() {
				for {
					reqs <- struct{}{}
				}
			}()
		}
	default:
		var mu sync.Mutex
		for i := 0; i < workers; i++ {
			go func() {
				for {
					mu.Lock()
					atomic.AddInt64(&counter, 1)
					mu.Unlock()
				}
			}()
		}
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var last int64
	for range ticker.C {
		cur := atomic.LoadInt64(&counter)
		fmt.Printf("ops/sec=%d goroutines=%d\n", cur-last, runtime.NumGoroutine())
		last = cur
	}
}
