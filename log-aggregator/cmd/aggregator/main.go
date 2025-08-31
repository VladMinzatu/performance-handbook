package main

import (
	"os"
	"os/signal"

	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/model"
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/output"
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/receiver"
)

const bufferSize = 100
const socketPath = "/tmp/log.sock"
const outputFilePath = "aggregated_logs.jsonl"

func main() {
	events := make(chan model.LogEntry, bufferSize)
	done := make(chan struct{})

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		close(events)
	}()

	go launchFileOutputCollector(events, done)
	go launchUnixSocketReceiver(events)

	<-done
}

func launchFileOutputCollector(events <-chan model.LogEntry, done chan struct{}) {
	out := output.NewFileOutput(outputFilePath)
	defer close(done)
	if err := out.Write(events); err != nil {
		panic(err)
	}
}

func launchUnixSocketReceiver(events chan<- model.LogEntry) {
	receiver := receiver.NewUnixSocketReceiver(socketPath, "local-socket")
	if err := receiver.Receive(events); err != nil {
		panic(err)
	}
}
