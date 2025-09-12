package main

import (
	"fmt"
	"os"

	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/ipc"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <ipc-type> <message-size>\n", os.Args[0])
		os.Exit(1)
	}
	ipcType := os.Args[1]

	messageSize := 0
	_, err := fmt.Sscanf(os.Args[2], "%d", &messageSize)
	if err != nil || messageSize <= 0 {
		fmt.Fprintf(os.Stderr, "Invalid message size: %s\n", os.Args[2])
		os.Exit(1)
	}

	prod, ok := ipc.GetProducer(ipcType, messageSize)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown IPC type: %s\n", ipcType)
		os.Exit(1)
	}

	prod.Run()
}
