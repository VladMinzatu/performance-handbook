// Fork-storm workload for lab 0302.
//
// Every INTERVAL_MS it tries to start one long-lived child (`sleep
// CHILD_SECS`) via a raw fork+exec, logging each success and each refusal.
// Once the cgroup's pids.max is reached, fork fails with EAGAIN and the
// storm keeps trying anyway; as children exit and free slots, spawns
// succeed again, so the alive count saw-tooths just under the limit.
//
// Children are reaped with a non-blocking wait4 from the main loop rather
// than one blocked goroutine per child: a goroutine parked in wait4 pins
// an OS thread, and threads count against pids.max too - it would distort
// the very number under test. The Go runtime's own handful of threads
// still count, so leave headroom in pids.max above the number of children
// you expect.
package main

import (
	"errors"
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
	childSecs := strconv.Itoa(envInt("CHILD_SECS", 10))

	attr := &syscall.ProcAttr{Files: []uintptr{0, 1, 2}}
	alive, spawned, denied := 0, 0, 0
	lastReport := time.Now()

	for {
		// Reap any children that have exited.
		for {
			pid, err := syscall.Wait4(-1, nil, syscall.WNOHANG, nil)
			if pid <= 0 || err != nil {
				break
			}
			alive--
		}

		pid, err := syscall.ForkExec("/bin/sleep", []string{"sleep", childSecs}, attr)
		switch {
		case err == nil:
			alive++
			spawned++
			fmt.Printf("spawned pid=%d alive=%d\n", pid, alive)
		case errors.Is(err, syscall.EAGAIN):
			denied++
			fmt.Printf("DENIED fork: %v alive=%d\n", err, alive)
		default:
			fmt.Printf("fork failed unexpectedly: %v\n", err)
		}

		if time.Since(lastReport) >= time.Second {
			fmt.Printf("summary spawned=%d denied=%d alive=%d\n", spawned, denied, alive)
			lastReport = time.Now()
		}
		time.Sleep(interval)
	}
}
