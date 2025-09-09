package engine

import (
	"io"
	"log"
	"net"
	"os"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/VladMinzatu/performance-handbook/reverse-proxy/connector"
)

type Engine interface {
	Start()
	Serve(clientConn net.Conn, backend connector.BackendConnector) error
}

type GoroutineEngine struct{}

func (ge *GoroutineEngine) Start() {
	// No initialization needed for goroutine engine
}

func (ge *GoroutineEngine) Serve(clientConn net.Conn, backend connector.BackendConnector) error {
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
	return nil
}

type EpollEngine struct {
	epfd  int
	conns map[int]*ProxyConn
	mu    sync.Mutex
}

type ProxyConn struct {
	clientFd   int
	backendFd  int
	clientBuf  []byte
	backendBuf []byte
}

func NewEpollEngine() (*EpollEngine, error) {
	epfd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &EpollEngine{
		epfd:  epfd,
		conns: make(map[int]*ProxyConn),
	}, nil
}

func (e *EpollEngine) Start() {
	go e.Loop()
}

func (e *EpollEngine) Serve(client net.Conn, backend connector.BackendConnector) error {
	backendConn, err := backend.Get()
	if err != nil {
		log.Printf("backend connect failed: %v", err)
		return err
	}
	defer backend.Return(backendConn)

	clientFd, err := fdFromConn(client)
	if err != nil {
		return err
	}
	backendFd, err := fdFromConn(backendConn)
	if err != nil {
		return err
	}

	pc := &ProxyConn{
		clientFd:   clientFd,
		backendFd:  backendFd,
		clientBuf:  make([]byte, 32*1024),
		backendBuf: make([]byte, 32*1024),
	}

	e.mu.Lock()
	e.conns[clientFd] = pc
	e.conns[backendFd] = pc
	e.mu.Unlock()

	ev := &unix.EpollEvent{Events: unix.EPOLLIN | unix.EPOLLOUT | unix.EPOLLET}
	ev.Fd = int32(clientFd)
	if err := unix.EpollCtl(e.epfd, unix.EPOLL_CTL_ADD, clientFd, ev); err != nil {
		return err
	}
	ev.Fd = int32(backendFd)
	if err := unix.EpollCtl(e.epfd, unix.EPOLL_CTL_ADD, backendFd, ev); err != nil {
		return err
	}

	return nil
}

func (e *EpollEngine) Loop() {
	events := make([]unix.EpollEvent, 128)
	for {
		n, err := unix.EpollWait(e.epfd, events, -1)
		if err != nil && err != unix.EINTR {
			log.Printf("epoll wait error: %v", err)
			continue
		}
		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)
			e.handleEvent(fd, events[i].Events)
		}
	}
}

func (e *EpollEngine) handleEvent(fd int, events uint32) {
	e.mu.Lock()
	pc, ok := e.conns[fd]
	e.mu.Unlock()
	if !ok {
		return
	}

	var srcFd, dstFd int
	var buf []byte

	if fd == pc.clientFd {
		srcFd, dstFd, buf = pc.clientFd, pc.backendFd, pc.clientBuf
	} else {
		srcFd, dstFd, buf = pc.backendFd, pc.clientFd, pc.backendBuf
	}

	if events&unix.EPOLLIN != 0 {
		n, err := unix.Read(srcFd, buf)
		if err != nil {
			log.Printf("read error fd %d: %v", srcFd, err)
			e.closeConn(pc)
			return
		}
		if n == 0 {
			e.closeConn(pc)
			return
		}
		_, err = unix.Write(dstFd, buf[:n])
		if err != nil {
			log.Printf("write error fd %d: %v", dstFd, err)
			e.closeConn(pc)
			return
		}
	}
}

func (e *EpollEngine) closeConn(pc *ProxyConn) {
	unix.Close(pc.clientFd)
	unix.Close(pc.backendFd)

	e.mu.Lock()
	delete(e.conns, pc.clientFd)
	delete(e.conns, pc.backendFd)
	e.mu.Unlock()
}

func fdFromConn(c net.Conn) (int, error) {
	tcpConn, ok := c.(*net.TCPConn)
	if !ok {
		return -1, os.ErrInvalid
	}
	f, err := tcpConn.File()
	if err != nil {
		return -1, err
	}

	// Dup so we can safely close net.Conn without losing fd
	fd := int(f.Fd())
	unix.SetNonblock(fd, true)
	return fd, nil
}
