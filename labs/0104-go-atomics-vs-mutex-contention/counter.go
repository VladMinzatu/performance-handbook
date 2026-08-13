// Shared-counter workload for the atomics-vs-mutex contention lab.
// WORKERS goroutines all increment the same counter as fast as they can,
// forever - using either a lock-free atomic instruction (MODE=atomic) or
// a sync.Mutex-protected increment (MODE=mutex). Same logical operation,
// same contention (WORKERS goroutines hammering the same shared state);
// the only variable is the mechanism.
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
	mode := os.Getenv("MODE")
	if mode == "" {
		mode = "atomic"
	}
	workers := envInt("WORKERS", 32)

	fmt.Printf("mode=%s workers=%d GOMAXPROCS=%d\n", mode, workers, runtime.GOMAXPROCS(0))

	var counter int64
	var mu sync.Mutex

	for i := 0; i < workers; i++ {
		go func() {
			for {
				if mode == "mutex" {
					mu.Lock()
					counter++
					mu.Unlock()
				} else {
					atomic.AddInt64(&counter, 1)
				}
			}
		}()
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
