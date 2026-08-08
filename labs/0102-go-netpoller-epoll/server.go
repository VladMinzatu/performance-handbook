// TCP echo-with-work server for the netpoller/epoll lab. Every accepted
// connection gets its own goroutine blocked in Read - the whole point of
// this lab is that thousands of these can be outstanding at once without
// thousands of OS threads, because the runtime multiplexes the blocking
// through a shared epoll instance instead of parking a thread per
// goroutine.
package main

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

// Per-message CPU work, so that a synchronized "wake every goroutine at
// once" burst has real work to schedule - enough to make GOMAXPROCS-bound
// queueing visible, not so much that a single request takes forever.
const workIterations = 2000

func doWork() byte {
	data := make([]byte, 32)
	for i := 0; i < workIterations; i++ {
		sum := sha256.Sum256(data)
		data[0] = sum[0]
	}
	return data[0]
}

func handleConn(conn net.Conn, active *int64) {
	defer conn.Close()
	atomic.AddInt64(active, 1)
	defer atomic.AddInt64(active, -1)

	buf := make([]byte, 1)
	for {
		if _, err := conn.Read(buf); err != nil {
			return
		}
		buf[0] = doWork()
		if _, err := conn.Write(buf); err != nil {
			return
		}
	}
}

func main() {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":9000"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("listen error:", err)
		os.Exit(1)
	}
	fmt.Printf("listening on %s GOMAXPROCS=%d\n", addr, runtime.GOMAXPROCS(0))

	var active int64
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fmt.Printf("goroutines=%d active_conns=%d\n", runtime.NumGoroutine(), atomic.LoadInt64(&active))
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("accept error:", err)
			continue
		}
		go handleConn(conn, &active)
	}
}
