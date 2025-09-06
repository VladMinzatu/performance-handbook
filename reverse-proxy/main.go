package main

import (
	"io"
	"log"
	"net"
)

func main() {
	listenAddr := ":8080"
	backendAddr := "127.0.0.1:9000"

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
		go handleConn(clientConn, backendAddr)
	}
}

func handleConn(clientConn net.Conn, backendAddr string) {
	defer clientConn.Close()

	backendConn, err := net.Dial("tcp", backendAddr)
	if err != nil {
		log.Printf("backend dial failed: %v", err)
		return
	}
	defer backendConn.Close()

	go io.Copy(backendConn, clientConn) // client → backend
	io.Copy(clientConn, backendConn)    // backend → client
}
