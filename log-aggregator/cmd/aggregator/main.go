package main

import (
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/model"
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/output"
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/receiver"
)

const bufferSize = 100
const socketPath = "/tmp/log.sock"
const outputFilePath = "aggregated_logs.jsonl"

func main() {
	events := make(chan model.LogEntry, bufferSize)

	receiver := receiver.NewUnixSocketReceiver(socketPath, "local-socket")
	if err := receiver.Receive(events); err != nil {
		panic(err)
	}

	go func() {
		out := output.NewFileOutput(outputFilePath)
		if err := out.Write(events); err != nil {
			panic(err)
		}
	}()
}
