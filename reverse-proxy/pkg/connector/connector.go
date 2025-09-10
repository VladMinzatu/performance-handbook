package connector

import (
	"net"

	"github.com/VladMinzatu/performance-handbook/reverse-proxy/pkg/pool"
)

type BackendConnector interface {
	Get() (net.Conn, error)
	Return(net.Conn)
}

type AlwaysDialConnector struct {
	backendAddr string
}

func NewAlwaysDialConnector(backendAddr string) *AlwaysDialConnector {
	return &AlwaysDialConnector{backendAddr: backendAddr}
}

func (adc *AlwaysDialConnector) Get() (net.Conn, error) {
	return net.Dial("tcp", adc.backendAddr)
}

func (adc *AlwaysDialConnector) Return(conn net.Conn) {
	conn.Close()
}

type PoolConnector struct {
	pool *pool.ConnPool
}

func NewPoolConnector(backendAddr string, size int) (*PoolConnector, error) {
	p, err := pool.NewConnPool(backendAddr, size)
	if err != nil {
		return nil, err
	}
	return &PoolConnector{pool: p}, nil
}

func (pc *PoolConnector) Get() (net.Conn, error) {
	return pc.pool.Get()
}

func (pc *PoolConnector) Return(conn net.Conn) {
	pc.pool.Return(conn)
}
