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
	Path string
}

func NewUnixSocketReceiver(path string) *UnixSocketReceiver {
	return &UnixSocketReceiver{Path: path}
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
				unmarshalAndWrite(payload, events)
			}
		}(conn)
	}
}

type UnixDatagramSocketReceiver struct {
	socketPath string
}

func NewUnixDatagramSocketReceiver(socketPath string) *UnixDatagramSocketReceiver {
	return &UnixDatagramSocketReceiver{socketPath: socketPath}
}

func (u *UnixDatagramSocketReceiver) Receive(events chan<- model.LogEntry) error {
	addr, err := net.ResolveUnixAddr("unixgram", u.socketPath)
	if err != nil {
		return err
	}
	conn, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	conn.SetReadBuffer(1 << 20) // 1MB

	buf := make([]byte, 8192)
	for {
		n, _, err := conn.ReadFromUnix(buf)
		if err != nil {
			return err
		}
		payload := string(buf[:n])
		unmarshalAndWrite(payload, events)
	}
}

func unmarshalAndWrite(payload string, events chan<- model.LogEntry) {
	var logEntry model.LogEntry
	if err := json.Unmarshal([]byte(payload), &logEntry); err == nil {
		events <- logEntry
	}
}
