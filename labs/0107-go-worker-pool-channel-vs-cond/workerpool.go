// Bounded worker-pool queue workload: PRODUCERS goroutines push items into
// a bounded queue of capacity QUEUE_CAP as fast as possible; CONSUMERS
// goroutines pop and "process" them (a no-op, so only the queueing/
// coordination cost is measured, not real work) - via one of two
// equivalent bounded-queue implementations:
//
//   - MECH=channel: a buffered Go channel of capacity QUEUE_CAP. Send
//     blocks when full, receive blocks when empty - the backing array and
//     the parking/waking of blocked senders/receivers are entirely the
//     runtime's (hchan.lock, sudog queues).
//   - MECH=cond: a hand-rolled ring buffer of capacity QUEUE_CAP, guarded
//     by a sync.Mutex, with *two* separate sync.Cond (notFull/notEmpty),
//     each woken with Signal - never Broadcast, so a Push only ever wakes
//     a blocked Pop and vice versa, no wasted wakeups by construction.
//
// Both implementations are correctly bounded, so produced/sec and
// consumed/sec converge to the same steady-state throughput regardless of
// mechanism - the interesting difference isn't raw throughput, it's what
// each blocked Push/Pop costs underneath: a channel's runtime handoff
// completes the transfer as part of waking the other side, while
// Cond.Wait has to re-acquire the mutex and the caller has to re-check its
// predicate after waking. See ../README.md.
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

// boundedQueue is a fixed-capacity ring buffer guarded by a sync.Mutex,
// with separate notFull/notEmpty condition variables so each Push/Pop
// signals only the side that could possibly be waiting on it.
type boundedQueue struct {
	mu       sync.Mutex
	notFull  *sync.Cond
	notEmpty *sync.Cond
	buf      []int64
	head     int
	count    int
}

func newBoundedQueue(capacity int) *boundedQueue {
	q := &boundedQueue{buf: make([]int64, capacity)}
	q.notFull = sync.NewCond(&q.mu)
	q.notEmpty = sync.NewCond(&q.mu)
	return q
}

func (q *boundedQueue) push(v int64) {
	q.mu.Lock()
	for q.count == len(q.buf) {
		q.notFull.Wait()
	}
	q.buf[(q.head+q.count)%len(q.buf)] = v
	q.count++
	q.mu.Unlock()
	q.notEmpty.Signal()
}

func (q *boundedQueue) pop() int64 {
	q.mu.Lock()
	for q.count == 0 {
		q.notEmpty.Wait()
	}
	v := q.buf[q.head]
	q.head = (q.head + 1) % len(q.buf)
	q.count--
	q.mu.Unlock()
	q.notFull.Signal()
	return v
}

func main() {
	mech := os.Getenv("MECH")
	if mech == "" {
		mech = "channel"
	}
	producers := envInt("PRODUCERS", 8)
	consumers := envInt("CONSUMERS", 8)
	queueCap := envInt("QUEUE_CAP", 16)

	fmt.Printf("mech=%s producers=%d consumers=%d queue_cap=%d GOMAXPROCS=%d\n",
		mech, producers, consumers, queueCap, runtime.GOMAXPROCS(0))

	var produced, consumed int64

	switch mech {
	case "cond":
		q := newBoundedQueue(queueCap)
		for i := 0; i < producers; i++ {
			go func() {
				for {
					q.push(1)
					atomic.AddInt64(&produced, 1)
				}
			}()
		}
		for i := 0; i < consumers; i++ {
			go func() {
				for {
					q.pop()
					atomic.AddInt64(&consumed, 1)
				}
			}()
		}
	default:
		mech = "channel"
		queue := make(chan int64, queueCap)
		for i := 0; i < producers; i++ {
			go func() {
				for {
					queue <- 1
					atomic.AddInt64(&produced, 1)
				}
			}()
		}
		for i := 0; i < consumers; i++ {
			go func() {
				for {
					<-queue
					atomic.AddInt64(&consumed, 1)
				}
			}()
		}
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastProduced, lastConsumed int64
	for range ticker.C {
		p := atomic.LoadInt64(&produced)
		c := atomic.LoadInt64(&consumed)
		fmt.Printf("produced/sec=%d consumed/sec=%d goroutines=%d\n",
			p-lastProduced, c-lastConsumed, runtime.NumGoroutine())
		lastProduced, lastConsumed = p, c
	}
}
