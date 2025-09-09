package engine

import (
	"io"
	"log"
	"net"

	"github.com/VladMinzatu/performance-handbook/reverse-proxy/connector"
)

type Engine interface {
	Serve(clientConn net.Conn, backend connector.BackendConnector)
}

type GoroutineEngine struct{}

func (ge *GoroutineEngine) Serve(clientConn net.Conn, backend connector.BackendConnector) {
	go func() {
		defer clientConn.Close()

		backendConn, err := backend.Get()
		if err != nil {
			log.Printf("backend connect failed: %v", err)
			return
		}
		defer backend.Return(backendConn)

		go io.Copy(backendConn, clientConn)
		io.Copy(clientConn, backendConn)
	}()
}
