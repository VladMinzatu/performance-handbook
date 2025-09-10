package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/VladMinzatu/performance-handbook/reverse-proxy/pkg/connector"
	"github.com/VladMinzatu/performance-handbook/reverse-proxy/pkg/engine"
)

func main() {
	listenAddr := ":8080"
	backendAddr := "127.0.0.1:9000"

	connectorType := flag.String("connector", "dial", "backend connector type (pool or dial) [default: dial]")
	engineType := flag.String("engine", "goroutine", "engine type (goroutine or epoll) [default: goroutine]")
	flag.Parse()

	if *connectorType == "" {
		fmt.Fprintln(os.Stderr, "Error: -connector flag is required (pool or dial)")
		flag.Usage()
		os.Exit(2)
	}

	backend, err := resolveConnector(backendAddr, *connectorType)
	if err != nil {
		log.Fatalf("failed to create connector: %v", err)
	}

	engine, err := resolveEngine(*engineType)
	if err != nil {
		log.Fatalf("failed to create engine: %v", err)
	}
	engine.Start()

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	log.Printf("Proxy listening on %s, forwarding to %s", listenAddr, backendAddr)

	for {
		clientConn, err := ln.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		engine.Serve(clientConn, backend)
	}
}

func resolveConnector(backendAddr string, connectorType string) (connector.BackendConnector, error) {
	switch connectorType {
	case "pool":
		return connector.NewPoolConnector(backendAddr, 10)
	case "dial":
		return connector.NewAlwaysDialConnector(backendAddr), nil
	default:
		fmt.Fprintf(os.Stderr, "Unknown connector type: %s\n", connectorType)
		os.Exit(2)
	}
	return nil, fmt.Errorf("unreachable")
}

func resolveEngine(engineType string) (engine.Engine, error) {
	switch engineType {
	case "goroutine":
		return &engine.GoroutineEngine{}, nil
	case "epoll":
		return engine.NewEpollEngine()
	default:
		fmt.Fprintf(os.Stderr, "Unknown engine type: %s\n", engineType)
		os.Exit(2)
	}
	return nil, fmt.Errorf("unreachable")
}
