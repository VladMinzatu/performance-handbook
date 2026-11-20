// The Go server under study: a TCP push-gateway protocol. The client
// sends "SUB <topic>\n" and gets "OK\n"; after that the connection only
// carries "PING\n" / "PONG\n" and is otherwise idle.
//
// MODE picks how each connection is handled:
//
//	pumps  a read goroutine with a 4KB bufio.Reader, and a write goroutine
//	       draining a channel through a 4KB bufio.Writer: the shape of the
//	       common WebSocket read-pump/write-pump pattern (default)
//	lean   one goroutine waiting in Read with a 16-byte buffer; it takes a
//	       4KB buffer from a sync.Pool only while it has data to process,
//	       and writes replies straight to the socket
//
// Logs every 5s: connections, goroutines, and from runtime/metrics the
// memory in goroutine stacks and in heap objects (total and per
// connection), and the size new goroutine stacks currently start at.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"runtime/metrics"
	"sync"
	"sync/atomic"
	"time"
)

var (
	replyOK   = []byte("OK\n")
	replyPong = []byte("PONG\n")
	conns     atomic.Int64
)

func reply(line []byte) []byte {
	if bytes.HasPrefix(line, []byte("PING")) {
		return replyPong
	}
	return replyOK
}

// handlePumps: two goroutines per connection, each holding a 4KB buffer
// for the life of the connection.
func handlePumps(c net.Conn) {
	out := make(chan []byte, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		w := bufio.NewWriterSize(c, 4096)
		for msg := range out {
			if _, err := w.Write(msg); err != nil {
				return
			}
			if err := w.Flush(); err != nil {
				return
			}
		}
	}()

	r := bufio.NewReaderSize(c, 4096)
	for {
		line, err := r.ReadSlice('\n')
		if err != nil {
			break
		}
		out <- reply(line)
	}
	close(out)
	<-done
}

var bufPool = sync.Pool{New: func() any { b := make([]byte, 0, 4096); return &b }}

// handleLean: one goroutine per connection. While idle it holds only a
// 16-byte read buffer; a 4KB buffer is borrowed only while a message is
// being read and answered.
func handleLean(c net.Conn) {
	var small [16]byte
	for {
		n, err := c.Read(small[:])
		if err != nil {
			return
		}
		bp := bufPool.Get().(*[]byte)
		buf := append((*bp)[:0], small[:n]...)
		// Read until the data ends on a complete line.
		for len(buf) > 0 && buf[len(buf)-1] != '\n' && err == nil {
			if len(buf) == cap(buf) {
				break // a line longer than the buffer; answer what we have
			}
			n, err = c.Read(buf[len(buf):cap(buf)])
			buf = buf[:len(buf)+n]
		}
		for _, line := range bytes.SplitAfter(buf, []byte("\n")) {
			if len(line) > 0 {
				if _, werr := c.Write(reply(line)); werr != nil {
					err = werr
				}
			}
		}
		*bp = buf[:0]
		bufPool.Put(bp)
		if err != nil {
			return
		}
	}
}

func main() {
	mode := os.Getenv("MODE")
	if mode == "" {
		mode = "pumps"
	}
	handle := handlePumps
	switch mode {
	case "pumps":
	case "lean":
		handle = handleLean
	default:
		log.Fatalf("unknown MODE %q (want pumps or lean)", mode)
	}

	ln, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("MODE=%s GOMAXPROCS=%d listening on :9000", mode, runtime.GOMAXPROCS(0))

	go report()
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			time.Sleep(10 * time.Millisecond)
			continue
		}
		conns.Add(1)
		go func() {
			defer conns.Add(-1)
			defer c.Close()
			handle(c)
		}()
	}
}

func report() {
	samples := []metrics.Sample{
		{Name: "/memory/classes/heap/stacks:bytes"},
		{Name: "/memory/classes/heap/objects:bytes"},
		{Name: "/gc/stack/starting-size:bytes"},
	}
	start := time.Now()
	for range time.Tick(5 * time.Second) {
		metrics.Read(samples)
		stacks := samples[0].Value.Uint64()
		heap := samples[1].Value.Uint64()
		startSize := samples[2].Value.Uint64()
		n := conns.Load()
		per := func(b uint64) string {
			if n == 0 {
				return "-"
			}
			return fmt.Sprintf("%.1fKB", float64(b)/float64(n)/1024)
		}
		fmt.Printf("t=%4.0fs conns=%6d goroutines=%6d stacks=%6.1fMB (%s/conn) heap=%6.1fMB (%s/conn) stack_start=%dB\n",
			time.Since(start).Seconds(), n, runtime.NumGoroutine(),
			float64(stacks)/(1<<20), per(stacks), float64(heap)/(1<<20), per(heap), startSize)
	}
}
