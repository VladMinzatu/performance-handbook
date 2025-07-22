package main

// implementation taken directly from the reference: https://github.com/cloudwego/netpoll-examples/blob/main/echo/server.go

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/netpoll"
	"github.com/pkg/profile"
)

func main() {
	defer profile.Start(profile.TraceProfile, profile.ProfilePath(".")).Stop()
	network, address := "tcp", ":8081"
	listener, _ := netpoll.CreateListener(network, address)

	eventLoop, _ := netpoll.NewEventLoop(
		handle,
		netpoll.WithOnPrepare(prepare),
		netpoll.WithOnConnect(connect),
		netpoll.WithReadTimeout(time.Second),
	)

	log.Printf("Netpoll echo server listening on port 8081")
	// start listen loop ...
	eventLoop.Serve(listener)
}

var _ netpoll.OnPrepare = prepare
var _ netpoll.OnConnect = connect
var _ netpoll.OnRequest = handle
var _ netpoll.CloseCallback = close

func prepare(connection netpoll.Connection) context.Context {
	return context.Background()
}

func close(connection netpoll.Connection) error {
	return nil
}

func connect(ctx context.Context, connection netpoll.Connection) context.Context {
	connection.AddCloseCallback(close)
	return ctx
}

func handle(ctx context.Context, connection netpoll.Connection) error {
	reader, writer := connection.Reader(), connection.Writer()
	defer reader.Release()

	msg, _ := reader.ReadString(reader.Len())

	writer.WriteString(msg)
	writer.Flush()

	return nil
}
