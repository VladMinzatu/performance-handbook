// Thread-growth workload for the blocking-syscalls-vs-network-I/O lab.
// Spawns WORKERS goroutines that each loop forever, blocking for WORK_MS
// on every iteration - but *how* they block depends on MODE, and that's
// the entire point:
//
//   - "timer":   time.Sleep - a pure runtime timer. The goroutine parks;
//     no syscall happens at all.
//   - "syscall": a raw nanosleep syscall, invoked directly instead of
//     through time.Sleep - a genuine blocking syscall that ties up the
//     underlying OS thread (M) for the duration.
//   - "netpoll": a network round trip against the companion delay-server,
//     which is running in a Read call, and is exactly the mechanism lab
//     0102 already showed costs no threads.
//
// Same wall-clock block time in all three modes; only the mechanism
// differs.
package main

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

func timerWorker(workMS int) {
	d := time.Duration(workMS) * time.Millisecond
	for {
		time.Sleep(d)
	}
}

func syscallWorker(workMS int) {
	ts := syscall.NsecToTimespec(int64(workMS) * int64(time.Millisecond))
	for {
		req := ts
		syscall.Nanosleep(&req, nil)
	}
}

func netpollWorker(addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Println("dial error:", err)
		return
	}
	defer conn.Close()
	buf := make([]byte, 1)
	for {
		if _, err := conn.Write(buf); err != nil {
			return
		}
		if _, err := conn.Read(buf); err != nil {
			return
		}
	}
}

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
		mode = "timer"
	}
	workers := envInt("WORKERS", 2000)
	workMS := envInt("WORK_MS", 200)
	addr := os.Getenv("DELAY_SERVER_ADDR")
	if addr == "" {
		addr = "delay-server:9100"
	}

	fmt.Printf("mode=%s workers=%d work_ms=%d GOMAXPROCS=%d\n", mode, workers, workMS, runtime.GOMAXPROCS(0))

	// Start workers in small staggered batches - mainly matters for
	// "netpoll" mode, to avoid a dial-storm against delay-server.
	const batch = 200
	for start := 0; start < workers; start += batch {
		end := start + batch
		if end > workers {
			end = workers
		}
		for i := start; i < end; i++ {
			go func() {
				switch mode {
				case "syscall":
					syscallWorker(workMS)
				case "netpoll":
					netpollWorker(addr)
				default:
					timerWorker(workMS)
				}
			}()
		}
		time.Sleep(50 * time.Millisecond)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		fmt.Printf("goroutines=%d\n", runtime.NumGoroutine())
	}
}
