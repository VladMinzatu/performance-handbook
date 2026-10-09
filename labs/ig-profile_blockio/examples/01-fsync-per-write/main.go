// An event-ingest service: POST /events appends the body as one line to
// /data/events.log and replies only once the line is on disk.
//
// SYNC=each  - every request writes its line and calls fsync before replying
// SYNC=group - requests queue their line; one writer writes everything queued
//              so far, calls fsync once, then replies to all of them
//
// Both settings reply only after the event is durable.
package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var (
	events atomic.Int64
	syncs  atomic.Int64
)

type pending struct {
	line []byte
	done chan error
}

func main() {
	mode := os.Getenv("SYNC")
	if mode == "" {
		mode = "each"
	}
	f, err := os.OpenFile("/data/events.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND|os.O_TRUNC, 0o644)
	if err != nil {
		log.Fatal(err)
	}

	var appendEvent func(line []byte) error
	switch mode {
	case "each":
		var mu sync.Mutex
		appendEvent = func(line []byte) error {
			mu.Lock()
			defer mu.Unlock()
			if _, err := f.Write(line); err != nil {
				return err
			}
			syncs.Add(1)
			return f.Sync()
		}
	case "group":
		queue := make(chan pending, 4096)
		go groupWriter(f, queue)
		appendEvent = func(line []byte) error {
			p := pending{line: line, done: make(chan error, 1)}
			queue <- p
			return <-p.done
		}
	default:
		log.Fatalf("unknown SYNC=%q (want each or group)", mode)
	}

	http.HandleFunc("POST /events", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := appendEvent(append(body, '\n')); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		events.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})

	go report()
	log.Printf("SYNC=%s listening on :8080", mode)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// groupWriter takes whatever is queued, writes it in one go, and covers
// all of it with a single fsync. Requests that arrive while an fsync is
// in progress form the next batch.
func groupWriter(f *os.File, queue chan pending) {
	var buf []byte
	var batch []pending
	for p := range queue {
		batch = append(batch[:0], p)
	drain:
		for len(batch) < cap(queue) {
			select {
			case p := <-queue:
				batch = append(batch, p)
			default:
				break drain
			}
		}
		buf = buf[:0]
		for _, p := range batch {
			buf = append(buf, p.line...)
		}
		_, err := f.Write(buf)
		if err == nil {
			syncs.Add(1)
			err = f.Sync()
		}
		for _, p := range batch {
			p.done <- err
		}
	}
}

func report() {
	start := time.Now()
	var lastEvents, lastSyncs int64
	for range time.Tick(5 * time.Second) {
		e, s := events.Load(), syncs.Load()
		de, ds := e-lastEvents, s-lastSyncs
		lastEvents, lastSyncs = e, s
		perSync := 0.0
		if ds > 0 {
			perSync = float64(de) / float64(ds)
		}
		log.Printf("t=%4.0fs events/s=%7.0f fsyncs/s=%6.0f events/fsync=%6.1f",
			time.Since(start).Seconds(), float64(de)/5, float64(ds)/5, perSync)
	}
}
