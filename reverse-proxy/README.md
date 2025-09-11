# reverse-proxy

Simple TCP reverse-proxy where I aim to implement and explore multiple variations for handling connections.

The server takes 2 configuration parameters. The first one is `connector`, which controls how the reverse-proxy connects to the backend. It has 2 possible values:
- **dial**: will dial a new TCP connection to the backend for each client connection accepted.
- **pool**: uses a pool of connections to the backend. For each client connection accepted, a connection is borrowed from the pool for forwarding traffic and it is returned to the pool when the client connection is closed.

The second configuration flag is `engine` and it also has 2 possible values:
- **goroutine**: will spin up a goroutine for the handling of each incoming client connection until it is closed.
- **epoll**: will roll out its own low level event loop using Linux `epoll` bypassing the Go netpoller. This should avoid the memory and scheduling overhead of the goroutine-per-connection model.

All 4 possible combinations of these flag values are possible for testing.