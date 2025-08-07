package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

type Stats struct {
	mu        sync.Mutex
	sent      int
	received  int
	failures  int
	totalRTT  time.Duration
	histogram *hdrhistogram.Histogram
}

func NewStats() *Stats {
	// Track latencies from 1 microsecond to 10 seconds, with 3 significant figures
	return &Stats{
		histogram: hdrhistogram.New(1, 10_000_000_000, 3),
	}
}

func (s *Stats) Record(rtt time.Duration, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent++
	if ok {
		s.received++
		s.totalRTT += rtt
		_ = s.histogram.RecordValue(rtt.Nanoseconds())
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
	idle := flag.Bool("idle", false, "If set, open long-lived mostly idle connections (no traffic except keepalive)")
	keepaliveInterval := flag.Duration("keepalive-interval", 1*time.Second, "Interval for keepalive pings in idle mode (if idle is set)")
	flag.Parse()

	if *idle {
		fmt.Printf("Starting idle test: %d users for %s (keepalive every %s)\n", *users, *duration, *keepaliveInterval)
	} else {
		fmt.Printf("Starting test: %d users for %s\n", *users, *duration)
	}

	var wg sync.WaitGroup
	stats := NewStats()
	stop := make(chan struct{})

	for i := 0; i < *users; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rand.Seed(time.Now().UnixNano() + int64(id)) // jitter

			conn, err := net.Dial("tcp", *host)
			if err != nil {
				fmt.Println("Error connecting to server. User aborting: ", err)
				return // This user gives up and doesn't take part in the test if it can't connect at all
			}
			defer conn.Close()

			sendAndReceive := func(message string) bool {
				start := time.Now()
				_, err = conn.Write([]byte(message))
				if err != nil {
					fmt.Printf("Error writing to server: %v\n", err)
					stats.Record(0, false)
					return false
				}
				buf := make([]byte, 1024)
				conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, err = conn.Read(buf)
				rtt := time.Since(start)
				if err != nil {
					stats.Record(0, false)
					fmt.Printf("Error reading from server: %v\n", err)
					return false
				} else {
					stats.Record(rtt, true)
				}
				return true
			}

			if *idle {
				// Idle mode: just keep the connection open, send a keepalive ping every keepaliveInterval
				ticker := time.NewTicker(*keepaliveInterval)
				defer ticker.Stop()
				for {
					select {
					case <-stop:
						return
					case <-ticker.C:
						if !sendAndReceive("ping\n") {
							return
						}
					}
				}
			} else {
				// Normal load test mode
				for {
					select {
					case <-stop:
						return
					default:
						if !sendAndReceive(*msg + "\n") {
							return
						}
						if *interval > 0 {
							time.Sleep(*interval)
						}
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

		// Percentiles
		stats.mu.Lock()
		fmt.Printf("p90 RTT:  %s\n", time.Duration(stats.histogram.ValueAtQuantile(90.0)))
		fmt.Printf("p95 RTT:  %s\n", time.Duration(stats.histogram.ValueAtQuantile(95.0)))
		fmt.Printf("p99 RTT:  %s\n", time.Duration(stats.histogram.ValueAtQuantile(99.0)))
		stats.mu.Unlock()
	}
}
