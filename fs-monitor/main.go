package main

import (
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: tracker <dir>")
	}
	dir := os.Args[1]

	tracker, err := NewTracker(dir)
	if err != nil {
		log.Fatal(err)
	}
	tracker.Run()
}
