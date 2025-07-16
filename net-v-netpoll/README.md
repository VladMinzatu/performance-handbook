# net-v-netpoll

In this directory we have the code and results of running a series of tests comparing [Netpoll](https://github.com/cloudwego/netpoll) with Go's standard network stack for a series of different use cases.

Essentially, the standard library networking model is designed for blocking I/O APIs, so you can only follow the one-goroutine-per-connection model (although Go uses epoll under the hood for network calls). For high concurrency scenarios this adds overhead in the memory footprint of goroutines (small as it may be) and context switching. [Netpoll](https://github.com/cloudwego/netpoll) aims to fix that by using event-driven non-blocking I/O. No goroutines are blocked by I/O and I/O is handled asynchronously: when data is ready, you handle it and you can defer creating new goroutines only to when CPU-bound logic needs to run. This gives more fine grained control in handling concurrency and should lead to better performance for scenarios of long-lived connections or bursty or high volume traffic.

In this series, I want to generate the metrics to support and quantify those claims under varying workload types.
