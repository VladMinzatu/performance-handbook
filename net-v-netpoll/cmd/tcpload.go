package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

type Stats struct {
	mu       sync.Mutex
	sent     int
	received int
	failures int
	totalRTT time.Duration
}

func (s *Stats) Record(rtt time.Duration, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent++
	if ok {
		s.received++
		s.totalRTT += rtt
	} else {
		s.failures++
	}
}

func main() {
	// Flags
	host := flag.String("host", "127.0.0.1:8080", "Target host:port")
	users := flag.Int("users", 10, "Number of concurrent clients")
	duration := flag.Duration("duration", 10*time.Second, "Test duration")
	msg := flag.String("message", "hello", "Message to send")
	interval := flag.Duration("interval", 0, "Interval between sends per client")
	flag.Parse()

	fmt.Printf("Starting test: %d users for %s\n", *users, *duration)

	var wg sync.WaitGroup
	stats := &Stats{}
	stop := make(chan struct{})

	for i := 0; i < *users; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rand.Seed(time.Now().UnixNano() + int64(id)) // for jitter

			conn, err := net.Dial("tcp", *host)
			if err != nil {
				fmt.Println("Error connecting to server. User aborting: ", err)
				return // This user gives up and doesn't take part in the test if it can't connect at all
			}
			defer conn.Close()

			for {
				select {
				case <-stop:
					return
				default:
					start := time.Now()
					_, err = conn.Write([]byte(*msg + "\n"))
					if err != nil {
						fmt.Println("Error writing to server:", err)
						stats.Record(0, false)
						return
					}

					buf := make([]byte, 1024)
					conn.SetReadDeadline(time.Now().Add(2 * time.Second))
					_, err = conn.Read(buf)
					rtt := time.Since(start)
					if err != nil {
						stats.Record(0, false)
						fmt.Println("Error reading from server:", err)
						return
					} else {
						stats.Record(rtt, true)
					}

					if *interval > 0 {
						time.Sleep(*interval)
					}
				}
			}
		}(i)
	}

	// Wait for duration
	time.Sleep(*duration)
	close(stop)
	wg.Wait()

	// Final stats
	fmt.Println("\n=== Load Test Complete ===")
	fmt.Printf("Sent:     %d\n", stats.sent)
	fmt.Printf("Received: %d\n", stats.received)
	fmt.Printf("Failures: %d\n", stats.failures)
	if stats.received > 0 {
		avgRTT := stats.totalRTT / time.Duration(stats.received)
		fmt.Printf("Avg RTT:  %s\n", avgRTT)
	}
}
