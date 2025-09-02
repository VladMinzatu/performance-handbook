package ipc

import (
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/publisher"
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/receiver"
)

const socketPath = "/tmp/log.sock"
const outputFilePath = "aggregated_logs.jsonl"
const fifoPath = "/tmp/log_fifo"
const networkAddress = "127.0.0.1:9000"

type IPC struct {
	producer   *Producer
	aggregator *Aggregator
}

var ipcTypes = map[string]IPC{
	"unixsock": {
		producer:   NewProducer(publisher.NewUnixSocketPublisher(socketPath)),
		aggregator: NewAggregator(receiver.NewUnixSocketReceiver(socketPath)),
	},
	"unixgram": {
		producer:   NewProducer(publisher.NewUnixDatagramSocketPublisher(socketPath)),
		aggregator: NewAggregator(receiver.NewUnixDatagramSocketReceiver(socketPath)),
	},
	"fifo": {
		producer:   NewProducer(publisher.NewFIFOPublisher(fifoPath)),
		aggregator: NewAggregator(receiver.NewFIFOReceiver(fifoPath)),
	},
	"tcp": {
		producer:   NewProducer(publisher.NewTCPSocketPublisher(networkAddress)),
		aggregator: NewAggregator(receiver.NewTCPSocketReceiver(networkAddress)),
	},
	"udp": {
		producer:   NewProducer(publisher.NewUDPSocketPublisher(networkAddress)),
		aggregator: NewAggregator(receiver.NewUDPSocketReceiver(networkAddress)),
	},
}

func GetAggregator(ipcType string) (*Aggregator, bool) {
	ipc, exists := getIPC(ipcType)
	if !exists || ipc.aggregator == nil {
		return nil, false
	}
	return ipc.aggregator, true
}

func GetProducer(ipcType string) (*Producer, bool) {
	ipc, exists := getIPC(ipcType)
	if !exists || ipc.producer == nil {
		return nil, false
	}
	return ipc.producer, true
}

func getIPC(ipcType string) (IPC, bool) {
	ipc, exists := ipcTypes[ipcType]
	return ipc, exists
}
