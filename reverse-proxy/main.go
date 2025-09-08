package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/VladMinzatu/performance-handbook/reverse-proxy/connector"
)

func main() {
	listenAddr := ":8080"
	backendAddr := "127.0.0.1:9000"

	backend, err := resolveConnector(backendAddr)
	if err != nil {
		log.Fatalf("failed to create connector: %v", err)
	}

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
		go handleConn(clientConn, backend)
	}
}

func handleConn(clientConn net.Conn, backend connector.BackendConnector) {
	defer clientConn.Close()

	backendConn, err := backend.Get()
	if err != nil {
		log.Printf("backend connect failed: %v", err)
		return
	}
	defer backend.Return(backendConn)

	go io.Copy(backendConn, clientConn)
	io.Copy(clientConn, backendConn)
}

func resolveConnector(backendAddr string) (connector.BackendConnector, error) {
	connectorType := flag.String("connector", "", "backend connector type (pool or dial) [required]")
	flag.Parse()

	if *connectorType == "" {
		fmt.Fprintln(os.Stderr, "Error: -connector flag is required (pool or dial)")
		flag.Usage()
		os.Exit(2)
	}

	switch *connectorType {
	case "pool":
		return connector.NewPoolConnector(backendAddr, 10)
	case "dial":
		return connector.NewAlwaysDialConnector(backendAddr), nil
	default:
		fmt.Fprintf(os.Stderr, "Unknown connector type: %s\n", *connectorType)
		os.Exit(2)
	}
	return nil, fmt.Errorf("unreachable")
}
