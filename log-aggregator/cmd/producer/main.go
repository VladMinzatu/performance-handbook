package main

import (
	"fmt"
	"os"

	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/ipc"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <ipc-type>\n", os.Args[0])
		os.Exit(1)
	}
	ipcType := os.Args[1]

	prod, ok := ipc.GetProducer(ipcType)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown IPC type: %s\n", ipcType)
		os.Exit(1)
	}

	prod.Run()
}
