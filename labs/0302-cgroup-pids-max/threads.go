// Thread-heavy workload for lab 0302: trips pids.max without any exec.
//
// Every INTERVAL_MS it starts one more goroutine that blocks in a raw
// read(2) on a pipe nobody writes to. A goroutine blocked in a real
// syscall pins its OS thread (the runtime hands the P off and spins up
// another M for the rest of the program), so N blocked goroutines cost
// about N threads - and every thread is a task that counts against
// pids.max. Raw syscall.Read on a blocking fd is used deliberately:
// os.File on a pipe would go through the netpoller and park cheaply
// instead.
//
// When the cgroup refuses the next thread, the Go runtime has no way to
// degrade gracefully: it aborts with "runtime: failed to create new OS
// thread" and the process exits. That is how a pids limit looks from
// inside a Go program.
package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
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
	interval := time.Duration(envInt("INTERVAL_MS", 100)) * time.Millisecond
	max := envInt("GOROUTINES", 200)

	var p [2]int
	if err := syscall.Pipe(p[:]); err != nil {
		fmt.Println("pipe:", err)
		os.Exit(1)
	}

	for blocked := 1; blocked <= max; blocked++ {
		go func() {
			buf := make([]byte, 1)
			syscall.Read(p[0], buf) // blocks forever; pins an OS thread
		}()
		fmt.Printf("blocked=%d\n", blocked)
		time.Sleep(interval)
	}
	fmt.Println("reached GOROUTINES without being refused")
	select {}
}
