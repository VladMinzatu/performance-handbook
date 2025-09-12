package ipc

import (
	"sync"
	"time"

	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/model"
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/publisher"
)

const producerBufferSize = 100
const duration = 5 * time.Second
const frequency = 10 // events per second

type Producer struct {
	publisher   publisher.Publisher
	messageSize int
}

func NewProducer(publisher publisher.Publisher, messageSize int) *Producer {
	return &Producer{publisher: publisher, messageSize: messageSize}
}

func (p Producer) Run() {
	// Note: Using a buffered channel here to decouple the log event producer from our multiple publisher implementations, which gives us parallelism around blocking system calls, but also a buffer size to tune.
	// The alternative approach would be to give the publisher Start() and Close() methods and invoke a Publish() method on each event. That introduces runtime coupling, but cuts scheduling overhead.
	// An interesting thing to test and benchmark perhaps.
	events := make(chan model.LogEntry, producerBufferSize)

	// Note 2: Using a WaitGroup here to ensure our spawned goroutine joins before we exit.
	// This is a different approach than the Aggregator which uses a done channel passed into the goroutine.
	// Not sure which is cleaner, but I find it interesting to demonstrate multiple ways to skin a cat.
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		p.publisher.Publish(events)
	}()

	msg := MessageOfSize(p.messageSize)

	ticker := time.NewTicker(time.Second / time.Duration(frequency))
	defer ticker.Stop()
	timeout := time.After(duration)
	for {
		select {
		case <-timeout:
			close(events)
			wg.Wait()
			return
		case t := <-ticker.C:
			entry := model.LogEntry{
				Source:    "producer",
				Timestamp: t.Unix(),
				Level:     "INFO",
				Message:   msg,
			}
			events <- entry
		}
	}
}

func MessageOfSize(size int) string {
	buf := make([]byte, size)
	for i := 0; i < size; i++ {
		buf[i] = 'A' + byte(i%26)
	}
	return string(buf)
}
