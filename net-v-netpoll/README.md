# net-v-netpoll

In this directory we have the code and results of running a series of tests comparing [Netpoll](https://github.com/cloudwego/netpoll) with Go's standard network stack for a series of different use cases.

Essentially, the standard library networking model is designed for blocking I/O APIs, so you can only follow the one-goroutine-per-connection model (although Go uses epoll under the hood for network calls). For high concurrency scenarios this adds overhead in the memory footprint of goroutines (small as it may be) and context switching. [Netpoll](https://github.com/cloudwego/netpoll) aims to fix that by using event-driven non-blocking I/O. No goroutines are blocked by I/O and I/O is handled asynchronously: when data is ready, you handle it and you can defer creating new goroutines only to when CPU-bound logic needs to run. This gives more fine grained control in handling concurrency and should lead to better performance for scenarios of long-lived connections or bursty or high volume traffic.

In this series, I want to generate the metrics to support and quantify those claims under varying workload types.

## Simple Echo Server

We'll first have a look at simple echo server implementations using the `net` package vs `netpoll`.

Under the `cmd` directory we have `netpoll_echo` for our implementation of the echo server using netpoll, as well as a `std_echo` directory for the standard library implementation. And there is a simple custom Go TCP load testing tool inside `cmd/tcpload.go`, as this seems like the most straightforward way to test our TCP servers (most tools out there are http load testing tools).

For the first experiments, I'm just going to be running the tests directly on my Mac and using Go profiling and observability tools.

Let's first give the servers a simple spin and see what observations our simple load testing tool collects. First, start the servers in two separate terminals with the commands:

```
go run cmd/netpoll_echo/echo_server_netpoll.go
...
go run cmd/std_echo/echo_server_std.go
```

And then run the load test on the standard lib server:

```
go run tcpload.go --host 127.0.0.1:8080 --users 100 --duration 10s --message "hello" --interval 100ms
Starting test: 100 users for 10s

=== Load Test Complete ===
Sent:     9700
Received: 9700
Failures: 0
Avg RTT:  1.495764ms
```

And on the netpoll server:

```
Starting test: 100 users for 10s

=== Load Test Complete ===
Sent:     9700
Received: 9700
Failures: 0
Avg RTT:  1.568389ms
```
