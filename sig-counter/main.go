package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/profile"
)

func main() {
	defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGTERM, syscall.SIGINT)

	ticker := NewTicker()
	go ticker.Run(ctx)

	for {
		select {
		case s := <-sigs:
			switch s {
			case syscall.SIGUSR1:
				fmt.Printf("Ticker count: %d\n", ticker.Value())
			case syscall.SIGUSR2:
				fmt.Println("Resetting ticker count to zero")
				ticker.Reset()
			case syscall.SIGTERM, syscall.SIGINT:
				fmt.Println("received", s, "— shutting down")
				cancel()
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
