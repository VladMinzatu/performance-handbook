package publisher

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"

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

	write(conn, events)
}

type UnixDatagramSocketPublisher struct {
	socketPath string
}

func NewUnixDatagramSocketPublisher(socketPath string) *UnixDatagramSocketPublisher {
	return &UnixDatagramSocketPublisher{socketPath: socketPath}
}

func (u *UnixDatagramSocketPublisher) Publish(events <-chan model.LogEntry) {
	raddr, err := net.ResolveUnixAddr("unixgram", u.socketPath)
	if err != nil {
		panic(err)
	}
	conn, err := net.DialUnix("unixgram", nil, raddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	write(conn, events)
}

func write(conn io.Writer, events <-chan model.LogEntry) {
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

type FIFOPublisher struct {
	fifoPath string
}

func NewFIFOPublisher(fifoPath string) *FIFOPublisher {
	return &FIFOPublisher{fifoPath: fifoPath}
}

func (p *FIFOPublisher) Publish(events <-chan model.LogEntry) {
	f, err := os.OpenFile(p.fifoPath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		log.Fatal("open fifo for writing:", err)
	}
	defer f.Close()

	write(f, events)
}
