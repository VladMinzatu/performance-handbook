package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudwego/netpoll"
)

func main() {
	addr := flag.String("addr", "localhost:8081", "address to listen on")
	flag.Parse()

	stop, err := StartNetpollEchoServer(*addr)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}

	log.Printf("Netpoll echo server listening on %s", *addr)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down server...")
	stop()
	log.Println("Server stopped")
}

func StartNetpollEchoServer(addr string) (stop func(), err error) {
	listener, err := netpoll.CreateListener("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create netpoll listener: %w", err)
	}

	// handler for incoming connections
	onRequest := func(ctx context.Context, connection netpoll.Connection) error {
		buf := make([]byte, 4096)
		n, err := connection.Read(buf)
		if err != nil {
			return err
		}

		_, err = connection.Write(buf[:n])
		return err
	}

	eventLoop, err := netpoll.NewEventLoop(netpoll.OnRequest(onRequest))
	if err != nil {
		return nil, fmt.Errorf("failed to create event loop: %w", err)
	}

	go func() {
		if serveErr := eventLoop.Serve(listener); serveErr != nil {
			log.Printf("netpoll event loop stopped: %v", serveErr)
		}
	}()

	stop = func() {
		eventLoop.Shutdown(context.Background())
	}
	return stop, nil
}
