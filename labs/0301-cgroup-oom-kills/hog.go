// OOM-kill demo workload for lab 0301.
//
// Grows resident memory by allocating fixed-size chunks and writing to
// every page in each chunk - anonymous memory that's never written to is
// served from a shared zero page and never counts against a cgroup's
// memory.max, so a "hog" that doesn't actually touch what it allocates
// never triggers anything (see README).
//
// NAME labels the process in its own log output - useful when more than
// one instance runs in the same container. CAP_MB, if set and non-zero,
// stops growth once that many MB have been allocated; CAP_MB=0 (or
// unset) means grow forever.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const chunkMB = 10
const pageSize = 4096

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func main() {
	name := os.Getenv("NAME")
	if name == "" {
		name = "hog"
	}
	capMB := envInt("CAP_MB", 0)

	var held [][]byte
	heldMB := 0
	for {
		if capMB == 0 || heldMB < capMB {
			chunk := make([]byte, chunkMB*1024*1024)
			for i := 0; i < len(chunk); i += pageSize {
				chunk[i] = 1
			}
			held = append(held, chunk)
			heldMB += chunkMB
		}
		fmt.Printf("%s holding %d MB\n", name, heldMB)
		time.Sleep(200 * time.Millisecond)
	}
}
