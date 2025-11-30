package ipc

import (
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/publisher"
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/receiver"
)

const socketPath = "/tmp/log.sock"
const networkAddress = "127.0.0.1:9000"
const fifoPath = "/tmp/log_fifo"
const outputFilePath = "aggregated_logs.jsonl"
const defaultMessageSize = 100

type IPC struct {
	producer   *Producer
	aggregator *Aggregator
}

var ipcTypes = map[string]IPC{
	"unixsock": {
		producer:   NewProducer(publisher.NewUnixSocketPublisher(socketPath), defaultMessageSize),
		aggregator: NewAggregator(receiver.NewUnixSocketReceiver(socketPath)),
	},
	"tcp": {
		producer:   NewProducer(publisher.NewTCPSocketPublisher(networkAddress), defaultMessageSize),
		aggregator: NewAggregator(receiver.NewTCPSocketReceiver(networkAddress)),
	},
	"unixgram": {
		producer:   NewProducer(publisher.NewUnixDatagramSocketPublisher(socketPath), defaultMessageSize),
		aggregator: NewAggregator(receiver.NewUnixDatagramSocketReceiver(socketPath)),
	},
	"udp": {
		producer:   NewProducer(publisher.NewUDPSocketPublisher(networkAddress), defaultMessageSize),
		aggregator: NewAggregator(receiver.NewUDPSocketReceiver(networkAddress)),
	},
	"fifo": {
		producer:   NewProducer(publisher.NewFIFOPublisher(fifoPath), defaultMessageSize),
		aggregator: NewAggregator(receiver.NewFIFOReceiver(fifoPath)),
	},
}

func GetAggregator(ipcType string) (*Aggregator, bool) {
	ipc, exists := getIPC(ipcType)
	if !exists || ipc.aggregator == nil {
		return nil, false
	}
	return ipc.aggregator, true
}

func GetProducer(ipcType string, messageSize int) (*Producer, bool) {
	ipc, exists := getIPC(ipcType)
	if !exists || ipc.producer == nil {
		return nil, false
	}
	ipc.producer.messageSize = messageSize
	return ipc.producer, true
}

func getIPC(ipcType string) (IPC, bool) {
	ipc, exists := ipcTypes[ipcType]
	return ipc, exists
}
