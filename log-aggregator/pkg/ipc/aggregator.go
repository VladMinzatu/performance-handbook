package ipc

import (
	"os"
	"os/signal"

	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/model"
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/output"
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/receiver"
)

const aggregatorBufferSize = 100

type Aggregator struct {
	receiver receiver.Receiver
}

func NewAggregator(receiver receiver.Receiver) *Aggregator {
	return &Aggregator{
		receiver: receiver,
	}
}

func (u *Aggregator) Run() {
	events := make(chan model.LogEntry, aggregatorBufferSize)
	done := make(chan struct{})

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		close(events)
	}()

	go launchFileOutputCollector(events, done)
	go func() {
		if err := u.receiver.Receive(events); err != nil {
			panic(err)
		}
	}()

	<-done
}

func launchFileOutputCollector(events <-chan model.LogEntry, done chan struct{}) {
	out := output.NewFileOutput(outputFilePath)
	defer close(done)
	if err := out.Write(events); err != nil {
		panic(err)
	}
}
