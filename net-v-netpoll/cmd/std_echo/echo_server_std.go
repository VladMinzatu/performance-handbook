package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/profile"
)

func main() {
	defer profile.Start(profile.TraceProfile, profile.ProfilePath(".")).Stop()
	addr := flag.String("addr", "localhost:8080", "address to listen on")
	flag.Parse()

	stop, err := StartStdEchoServer(*addr)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}

	log.Printf("Standard echo server listening on %s", *addr)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	time.Sleep(1 * time.Second) // TODO: check why this is needed for the profile to be written

	log.Println("Shutting down server...")
	stop()
	log.Println("Server stopped")
}

func StartStdEchoServer(addr string) (stop func(), err error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	// Channel to signal shutdown
	stopChan := make(chan struct{})
	// For closing the listener safely
	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stopChan:
					return // graceful shutdown
				default:
					log.Printf("accept error: %v", err)
					continue
				}
			}
			go handleStdEchoConn(conn)
		}
	}()

	stop = func() {
		close(stopChan)
		ln.Close()
		<-doneChan
	}
	return stop, nil
}

func handleStdEchoConn(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		_, err = conn.Write(buf[:n])
		if err != nil {
			return
		}
	}
}
