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
	defer profile.Start(profile.TraceProfile, profile.ProfilePath(".")).Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGTERM, syscall.SIGINT)

	ticker := NewTicker()
	stop := make(chan struct{})

	go func() {
		ticker.Run(ctx)
		close(stop)
	}()

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
				<-stop // Wait for ticker to stop
				return
			}
		case <-ctx.Done():
			<-stop // Wait for ticker to stop
			return
		}
	}
}
