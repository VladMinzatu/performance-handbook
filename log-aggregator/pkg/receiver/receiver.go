package receiver

import (
	"bufio"
	"encoding/json"
	"net"

	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/model"
)

type Receiver interface {
	Receive(chan<- model.LogEntry) error
}

type UnixSocketReceiver struct {
	Path   string
	Source string
}

func NewUnixSocketReceiver(path, source string) *UnixSocketReceiver {
	return &UnixSocketReceiver{Path: path, Source: source}
}

func (u *UnixSocketReceiver) Receive(events chan<- model.LogEntry) error {
	ln, err := net.Listen("unix", u.Path)
	if err != nil {
		return err
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}

		go func(c net.Conn) {
			defer c.Close()
			scanner := bufio.NewScanner(c)
			for scanner.Scan() {
				payload := scanner.Text()
				var logEntry model.LogEntry
				if err := json.Unmarshal([]byte(payload), &logEntry); err == nil {
					events <- logEntry
					continue
				}
			}
		}(conn)
	}
}
