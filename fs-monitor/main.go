package main

import (
	"log"
	"os"

	"github.com/VladMinzatu/performance-handbook/fs-monitor/tracking"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: tracker <dir>")
	}
	dir := os.Args[1]

	tracker, err := tracking.NewTracker(dir)
	if err != nil {
		log.Fatal(err)
	}
	tracker.Run()
}
