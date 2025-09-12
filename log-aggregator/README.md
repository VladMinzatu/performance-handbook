# log-aggregator

In production systems, log capture is usually handled transparently by the runtime environment (e.g., Docker, containerd, or systemd), which redirects each process’s stdout/stderr into a managed logging pipeline. In this project, instead of relying on that layer, we experiment with different low-level techniques for local log aggregation — such as pipes, Unix domain sockets, and networking — to better understand the trade-offs of various IPC mechanisms before forwarding logs into a distributed system.

## The implementations

I implemented the log aggregation (the communication between the log producers and the aggregator) using a few different techniques that I want to then analyse:
- **Unix Domain Sockets**: connection-oriented, reliable and stream-oriented. Provides ordering and reliability guarantees. Similar to TCP, but confined to a single machine. We expect this to be quite performant: it just involves memory copy between processes. Some overhead comes from the need to explicitly set up and teardown connections, but that is not a significant factor for our use case here.
- **TCP**: also connection-oriented, reliable and stream-oriented, but with the overhead of traversing the networking stack (TCP/IP headers). The loopback optimization will help in our locally running use case, but it should still have some overhead compared to Unix sockets, which will be interesting to measure. On the plus side, that enables remote logging, but that's not relevant for our use case.
- **Unix Domain Datagram**: connectionless, unreliable and message-oriented. Duplication, loss and reordering are possible, but if reliability is not a huge concern, they might be more performant than Unix Domain Sockets. Will be interesting to check exactly how. 
- **UDP**: also connectionless, unreliable and message-oriented, but over the network stack, so we'll have some overhead from that again, but should be faster than TCP on loopback, but with some of the same downsides as for Unix Domain Datagram.
- **FIFO Pipe**: this is also a stream-oriented approach with blocking/non-blocking semantics similar to file I/O. It is more typically used in one-writer-one-reader scenarios and its particular semantics make it a bad fit for our application (without some special handling at least). For example, when all writers stop writing and close the pipe, this acts as an EOF to the reader. But it might be interesting to check its performance and understand how it works under the hood compared to the other techniques. Moving the data from one process to another should just involve kernel buffer copying, so it should be very performant.

## Running local tests

First, the aggregator needs to be launched, listening on one of the supported IPC types, e.g.:
```
cd cmd/aggregator
go build .
./aggregator unixsock
```

Multiple producers can be launched to send logs, using the same IPC type for the functionality to work end to end:
```
bash launch-producers.sh
```

The aggregator will write all received logs to a local file called `aggregated_logs.jsonl`.
