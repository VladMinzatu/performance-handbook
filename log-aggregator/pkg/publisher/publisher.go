package publisher

import (
	"encoding/json"
	"net"

	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/model"
)

type Publisher interface {
	Publish(<-chan model.LogEntry)
}

type UnixSocketPublisher struct {
	socketPath string
}

func NewUnixSocketPublisher(socketPath string) *UnixSocketPublisher {
	return &UnixSocketPublisher{socketPath: socketPath}
}

func (u *UnixSocketPublisher) Publish(events <-chan model.LogEntry) {
	conn, err := net.Dial("unix", u.socketPath)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	for entry := range events {
		b, err := json.Marshal(entry)
		if err != nil {
			panic(err)
		}
		_, err = conn.Write(append(b, '\n'))
		if err != nil {
			panic(err)
		}
	}
}
