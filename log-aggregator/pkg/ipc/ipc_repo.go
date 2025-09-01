package ipc

import (
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/publisher"
	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/receiver"
)

const socketPath = "/tmp/log.sock"
const outputFilePath = "aggregated_logs.jsonl"

type IPC struct {
	producer   *Producer
	aggregator *Aggregator
}

var ipcTypes = map[string]IPC{
	"unix_socket_stream": {
		producer:   NewProducer(publisher.NewUnixSocketPublisher(socketPath)),
		aggregator: NewAggregator(receiver.NewUnixSocketReceiver(socketPath)),
	},
	"unix_socket_datagram": {
		producer:   NewProducer(publisher.NewUnixDatagramSocketPublisher(socketPath)),
		aggregator: NewAggregator(receiver.NewUnixDatagramSocketReceiver(socketPath)),
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
