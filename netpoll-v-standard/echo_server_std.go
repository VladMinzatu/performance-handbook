package main

import (
	"fmt"
	"log"
	"net"
)

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
