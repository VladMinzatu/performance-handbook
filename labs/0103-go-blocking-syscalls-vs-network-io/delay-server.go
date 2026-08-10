// Companion server for "netpoll" mode: echoes one byte back after
// sleeping DELAY_MS, so a client-side goroutine blocked waiting for the
// reply is blocked in a network Read for a controlled, comparable
// duration to the other two modes' WORK_MS. The server side's own
// thread behavior isn't the point here - lab 0102 already covers that -
// this just needs to hold the client end blocked on the network for a
// known amount of time.
package main

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"time"
)

func main() {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":9100"
	}
	delayMS := 200
	if v := os.Getenv("DELAY_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			delayMS = n
		}
	}
	delay := time.Duration(delayMS) * time.Millisecond

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("listen error:", err)
		os.Exit(1)
	}
	fmt.Printf("delay-server listening on %s delay=%s GOMAXPROCS=%d\n", addr, delay, runtime.GOMAXPROCS(0))

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fmt.Printf("goroutines=%d\n", runtime.NumGoroutine())
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("accept error:", err)
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			buf := make([]byte, 1)
			for {
				if _, err := c.Read(buf); err != nil {
					return
				}
				time.Sleep(delay)
				if _, err := c.Write(buf); err != nil {
					return
				}
			}
		}(conn)
	}
}
