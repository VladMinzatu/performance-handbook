package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/model"
)

const socketPath = "/tmp/log.sock"

func main() {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	for i := 0; i < 5; i++ {
		entry := model.LogEntry{
			Source:    "producer",
			Timestamp: time.Now().Unix(),
			Level:     "INFO",
			Message:   fmt.Sprintf("producer log %d", i),
		}
		b, err := json.Marshal(entry)
		if err != nil {
			panic(err)
		}
		_, err = conn.Write(append(b, '\n'))
		if err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Second)
	}
}
