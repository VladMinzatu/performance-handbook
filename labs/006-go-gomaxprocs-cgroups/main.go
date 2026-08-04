// CPU-bound workload for the GOMAXPROCS-vs-cgroup-limit lab. Spawns exactly
// GOMAXPROCS busy-looping goroutines (each hashing repeatedly - real CPU
// work, not something the compiler can optimize away) and prints
// throughput once a second. The interesting variable isn't in this code at
// all: it's whether GOMAXPROCS actually matches the container's real CPU
// entitlement when this starts up.
package main

import (
	"crypto/sha256"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

func worker(counter *uint64, stop <-chan struct{}) {
	data := make([]byte, 64)
	for {
		select {
		case <-stop:
			return
		default:
			sum := sha256.Sum256(data)
			data[0] = sum[0]
			atomic.AddUint64(counter, 1)
		}
	}
}

func main() {
	numWorkers := runtime.GOMAXPROCS(0)
	fmt.Printf("GOMAXPROCS=%d NumCPU=%d numWorkers=%d\n", runtime.GOMAXPROCS(0), runtime.NumCPU(), numWorkers)

	var counter uint64
	stop := make(chan struct{})
	for i := 0; i < numWorkers; i++ {
		go worker(&counter, stop)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var last uint64
	for range ticker.C {
		cur := atomic.LoadUint64(&counter)
		fmt.Printf("ops/sec=%d goroutines=%d\n", cur-last, runtime.NumGoroutine())
		last = cur
	}
}
