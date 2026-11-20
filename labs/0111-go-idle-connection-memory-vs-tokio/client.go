// The load generator: opens CONNS connections to one server at RAMP new
// connections per second, subscribes on each, and then leaves each one
// idle apart from a "PING\n" every PING_EVERY, at a random offset so the
// pings are spread out evenly. It never reconnects: a connection that
// fails is counted and stays closed, so the server's connection count
// only goes down if something broke.
//
// Logs every 5s:
//
//	open        connections currently open (dialed, subscribed, not failed)
//	dial_err    dials or subscriptions that failed in the interval
//	reset       open connections that failed in the interval (a ping
//	            that couldn't be sent or wasn't answered)
//	pings       pings answered in the interval
//
// Knobs (env):
//
//	TARGET      go or tokio, or host:port (default go)
//	CONNS       connections to open (default 50000)
//	RAMP        new connections per second (default 2000)
//	PING_EVERY  interval between pings on each connection (default 30s)
package main

import (
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

var open, dialErrs, resets, pings atomic.Int64

func main() {
	target := os.Getenv("TARGET")
	switch target {
	case "", "go":
		target = "lab-go-idleconn-goserver:9000"
	case "tokio":
		target = "lab-go-idleconn-tokioserver:9000"
	}
	n := envInt("CONNS", 50000)
	ramp := envInt("RAMP", 2000)
	every, err := time.ParseDuration(os.Getenv("PING_EVERY"))
	if err != nil {
		every = 30 * time.Second
	}
	log.Printf("target=%s conns=%d ramp=%d/s ping_every=%s", target, n, ramp, every)

	go func() {
		start := time.Now()
		for range time.Tick(5 * time.Second) {
			fmt.Printf("t=%4.0fs open=%6d dial_err=%5d reset=%5d pings=%6d\n",
				time.Since(start).Seconds(), open.Load(), dialErrs.Swap(0), resets.Swap(0), pings.Swap(0))
		}
	}()

	// Open connections on a 10ms tick, however many are due by now.
	start := time.Now()
	dialed := 0
	for range time.Tick(10 * time.Millisecond) {
		for due := int(time.Since(start).Seconds()*float64(ramp)) - dialed; due > 0 && dialed < n; due-- {
			dialed++
			go hold(target, dialed, every)
		}
		if dialed >= n {
			break
		}
	}
	select {}
}

// hold dials, subscribes and then pings until something fails.
func hold(target string, id int, every time.Duration) {
	c, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		dialErrs.Add(1)
		return
	}
	defer c.Close()
	buf := make([]byte, 16)
	if err := roundTrip(c, []byte(fmt.Sprintf("SUB topic-%d\n", id%100)), buf); err != nil {
		dialErrs.Add(1)
		return
	}
	open.Add(1)
	time.Sleep(rand.N(every))
	for {
		if err := roundTrip(c, []byte("PING\n"), buf); err != nil {
			open.Add(-1)
			resets.Add(1)
			return
		}
		pings.Add(1)
		time.Sleep(every)
	}
}

// roundTrip sends msg and reads one reply line (both are a few bytes).
func roundTrip(c net.Conn, msg, buf []byte) error {
	c.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := c.Write(msg); err != nil {
		return err
	}
	for got := 0; ; {
		n, err := c.Read(buf[got:])
		if err != nil {
			if err == io.EOF {
				return io.ErrUnexpectedEOF
			}
			return err
		}
		got += n
		if buf[got-1] == '\n' || got == len(buf) {
			return nil
		}
	}
}
