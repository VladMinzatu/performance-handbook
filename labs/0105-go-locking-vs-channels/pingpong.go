// Pure hand-off workload for the locking-vs-channels lab: PARTICIPANTS
// goroutines arranged in a ring, passing a single token around and
// around, forever. No shared data structure is being protected and no
// "useful work" happens beyond counting the hand-off - this isolates the
// coordination/scheduling cost itself, independent of whatever it's
// usually bundled with (cache-line contention on real shared state,
// like in the counter workload or lab 0104).
//
//   - MECH=channel: a ring of unbuffered channels - the idiomatic Go
//     token-ring pattern. Each goroutine blocks receiving on its inbound
//     channel, then sends on the next goroutine's channel.
//   - MECH=mutex:   a shared "whose turn is it" integer protected by a
//     sync.Mutex, with a sync.Cond for blocking until it's this
//     goroutine's turn - the natural mutex-based way to express the same
//     blocking hand-off, since a bare Mutex alone has no way to block a
//     goroutine until a specific condition holds. Cond.Broadcast wakes
//     *every* waiter on each hand-off, only one of which will actually
//     find it's their turn - a real structural difference from the
//     channel ring, which wakes exactly one goroutine per hand-off.
//
// Since only one goroutine is ever doing anything at a time by
// construction (everyone else is blocked waiting for the token), this
// workload is inherently serialized regardless of GOMAXPROCS - a much
// lower-noise measurement of hand-off cost than a genuinely
// multi-core-contended workload would give.
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
	participants := envInt("PARTICIPANTS", 4)

	fmt.Printf("mech=%s participants=%d GOMAXPROCS=%d\n", mech, participants, runtime.GOMAXPROCS(0))

	var counter int64

	switch mech {
	case "channel":
		chans := make([]chan struct{}, participants)
		for i := range chans {
			chans[i] = make(chan struct{})
		}
		for i := 0; i < participants; i++ {
			go func(id int) {
				in := chans[id]
				out := chans[(id+1)%participants]
				for {
					<-in
					atomic.AddInt64(&counter, 1)
					out <- struct{}{}
				}
			}(i)
		}
		go func() { chans[0] <- struct{}{} }()
	default:
		var mu sync.Mutex
		cond := sync.NewCond(&mu)
		turn := 0
		for i := 0; i < participants; i++ {
			go func(id int) {
				for {
					mu.Lock()
					for turn != id {
						cond.Wait()
					}
					atomic.AddInt64(&counter, 1)
					turn = (turn + 1) % participants
					cond.Broadcast()
					mu.Unlock()
				}
			}(i)
		}
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var last int64
	for range ticker.C {
		cur := atomic.LoadInt64(&counter)
		fmt.Printf("handoffs/sec=%d goroutines=%d\n", cur-last, runtime.NumGoroutine())
		last = cur
	}
}
