package pool

import (
	"net"
)

type ConnPool struct {
	backendAddr string
	pool        chan net.Conn
}

func NewConnPool(backendAddr string, size int) (*ConnPool, error) {
	cp := &ConnPool{
		backendAddr: backendAddr,
		pool:        make(chan net.Conn, size),
	}
	for i := 0; i < size; i++ {
		conn, err := net.Dial("tcp", backendAddr)
		if err != nil {
			return nil, err
		}
		cp.pool <- conn
	}
	return cp, nil
}

func (cp *ConnPool) Get() (net.Conn, error) {
	conn := <-cp.pool
	return conn, nil
}

func (cp *ConnPool) Return(conn net.Conn) {
	select {
	case cp.pool <- conn:
	default:
		// Pool is full somehow - close excess connections
		conn.Close()
	}
}
