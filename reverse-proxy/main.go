package main

import (
	"io"
	"log"
	"net"

	"github.com/VladMinzatu/performance-handbook/reverse-proxy/connector"
)

func main() {
	listenAddr := ":8080"
	backendAddr := "127.0.0.1:9000"

	connector, err := connector.NewPoolConnector(backendAddr, 10)
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
		go handleConn(clientConn, connector)
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
