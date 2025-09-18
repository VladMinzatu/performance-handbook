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

## bpftrace

We'll conduct our behavior and performance analysis tests focusing on bpftrace (I am using `bpftrace v0.23.5` and the scripts are checked into the `.bpftrace/` directory).

First, while running our local tests, we'll run a simple syscall counter, like so:
```
cd bpftrace
sudo ./producer_syscalls.bt
```

which produces the following output for the various IPC types:

unixsock:
```
@[tracepoint:syscalls:sys_enter_faccessat]: 5
@[tracepoint:syscalls:sys_enter_connect]: 5
@[tracepoint:syscalls:sys_enter_getsockname]: 5
@[tracepoint:syscalls:sys_enter_exit_group]: 5
@[tracepoint:syscalls:sys_enter_eventfd2]: 5
@[tracepoint:syscalls:sys_enter_getrandom]: 5
@[tracepoint:syscalls:sys_enter_epoll_create1]: 5
@[tracepoint:syscalls:sys_enter_set_tid_address]: 5
@[tracepoint:syscalls:sys_enter_getpeername]: 5
@[tracepoint:syscalls:sys_enter_socket]: 5
@[tracepoint:syscalls:sys_enter_sched_yield]: 9
@[tracepoint:syscalls:sys_enter_sched_getaffinity]: 10
@[tracepoint:syscalls:sys_enter_madvise]: 10
@[tracepoint:syscalls:sys_enter_brk]: 15
@[tracepoint:syscalls:sys_enter_newfstat]: 15
@[tracepoint:syscalls:sys_enter_epoll_ctl]: 15
@[tracepoint:syscalls:sys_enter_prlimit64]: 20
@[tracepoint:syscalls:sys_enter_rt_sigreturn]: 22
@[tracepoint:syscalls:sys_enter_getpid]: 23
@[tracepoint:syscalls:sys_enter_tgkill]: 23
@[tracepoint:syscalls:sys_enter_clone3]: 26
@[tracepoint:syscalls:sys_enter_fcntl]: 30
@[tracepoint:syscalls:sys_enter_gettid]: 31
@[tracepoint:syscalls:sys_enter_rseq]: 31
@[tracepoint:syscalls:sys_enter_set_robust_list]: 31
@[tracepoint:syscalls:sys_enter_munmap]: 35
@[tracepoint:syscalls:sys_enter_close]: 35
@[tracepoint:syscalls:sys_enter_openat]: 35
@[tracepoint:syscalls:sys_enter_read]: 40
@[tracepoint:syscalls:sys_enter_sigaltstack]: 62
@[tracepoint:syscalls:sys_enter_mprotect]: 67
@[tracepoint:syscalls:sys_enter_rt_sigprocmask]: 171
@[tracepoint:syscalls:sys_enter_mmap]: 197
@[tracepoint:syscalls:sys_enter_write]: 250
@[tracepoint:syscalls:sys_enter_rt_sigaction]: 565
@[tracepoint:syscalls:sys_enter_epoll_pwait]: 590
@[tracepoint:syscalls:sys_enter_nanosleep]: 724
@[tracepoint:syscalls:sys_enter_futex]: 910
```

tcp:
```
@[tracepoint:syscalls:sys_enter_getsockopt]: 5
@[tracepoint:syscalls:sys_enter_epoll_create1]: 5
@[tracepoint:syscalls:sys_enter_faccessat]: 5
@[tracepoint:syscalls:sys_enter_connect]: 5
@[tracepoint:syscalls:sys_enter_set_tid_address]: 5
@[tracepoint:syscalls:sys_enter_socket]: 5
@[tracepoint:syscalls:sys_enter_getpeername]: 5
@[tracepoint:syscalls:sys_enter_getsockname]: 5
@[tracepoint:syscalls:sys_enter_eventfd2]: 5
@[tracepoint:syscalls:sys_enter_exit_group]: 5
@[tracepoint:syscalls:sys_enter_getrandom]: 5
@[tracepoint:syscalls:sys_enter_sched_getaffinity]: 10
@[tracepoint:syscalls:sys_enter_madvise]: 10
@[tracepoint:syscalls:sys_enter_epoll_ctl]: 15
@[tracepoint:syscalls:sys_enter_brk]: 15
@[tracepoint:syscalls:sys_enter_newfstat]: 15
@[tracepoint:syscalls:sys_enter_sched_yield]: 16
@[tracepoint:syscalls:sys_enter_prlimit64]: 20
@[tracepoint:syscalls:sys_enter_clone3]: 24
@[tracepoint:syscalls:sys_enter_rt_sigreturn]: 25
@[tracepoint:syscalls:sys_enter_setsockopt]: 25
@[tracepoint:syscalls:sys_enter_getpid]: 26
@[tracepoint:syscalls:sys_enter_tgkill]: 26
@[tracepoint:syscalls:sys_enter_rseq]: 29
@[tracepoint:syscalls:sys_enter_set_robust_list]: 29
@[tracepoint:syscalls:sys_enter_gettid]: 29
@[tracepoint:syscalls:sys_enter_fcntl]: 30
@[tracepoint:syscalls:sys_enter_close]: 35
@[tracepoint:syscalls:sys_enter_openat]: 35
@[tracepoint:syscalls:sys_enter_munmap]: 36
@[tracepoint:syscalls:sys_enter_read]: 40
@[tracepoint:syscalls:sys_enter_sigaltstack]: 58
@[tracepoint:syscalls:sys_enter_mprotect]: 63
@[tracepoint:syscalls:sys_enter_rt_sigprocmask]: 159
@[tracepoint:syscalls:sys_enter_mmap]: 191
@[tracepoint:syscalls:sys_enter_write]: 250
@[tracepoint:syscalls:sys_enter_epoll_pwait]: 339
@[tracepoint:syscalls:sys_enter_rt_sigaction]: 565
@[tracepoint:syscalls:sys_enter_nanosleep]: 803
@[tracepoint:syscalls:sys_enter_futex]: 955
```

unixgram:
```
@[tracepoint:syscalls:sys_enter_getrandom]: 5
@[tracepoint:syscalls:sys_enter_set_tid_address]: 5
@[tracepoint:syscalls:sys_enter_socket]: 5
@[tracepoint:syscalls:sys_enter_epoll_create1]: 5
@[tracepoint:syscalls:sys_enter_eventfd2]: 5
@[tracepoint:syscalls:sys_enter_faccessat]: 5
@[tracepoint:syscalls:sys_enter_getpeername]: 5
@[tracepoint:syscalls:sys_enter_connect]: 5
@[tracepoint:syscalls:sys_enter_getsockname]: 5
@[tracepoint:syscalls:sys_enter_exit_group]: 5
@[tracepoint:syscalls:sys_enter_sched_yield]: 7
@[tracepoint:syscalls:sys_enter_sched_getaffinity]: 10
@[tracepoint:syscalls:sys_enter_madvise]: 10
@[tracepoint:syscalls:sys_enter_brk]: 15
@[tracepoint:syscalls:sys_enter_newfstat]: 15
@[tracepoint:syscalls:sys_enter_epoll_ctl]: 15
@[tracepoint:syscalls:sys_enter_prlimit64]: 20
@[tracepoint:syscalls:sys_enter_tgkill]: 25
@[tracepoint:syscalls:sys_enter_rt_sigreturn]: 25
@[tracepoint:syscalls:sys_enter_clone3]: 25
@[tracepoint:syscalls:sys_enter_getpid]: 25
@[tracepoint:syscalls:sys_enter_rseq]: 30
@[tracepoint:syscalls:sys_enter_fcntl]: 30
@[tracepoint:syscalls:sys_enter_gettid]: 30
@[tracepoint:syscalls:sys_enter_set_robust_list]: 30
@[tracepoint:syscalls:sys_enter_close]: 35
@[tracepoint:syscalls:sys_enter_openat]: 35
@[tracepoint:syscalls:sys_enter_munmap]: 37
@[tracepoint:syscalls:sys_enter_read]: 40
@[tracepoint:syscalls:sys_enter_sigaltstack]: 60
@[tracepoint:syscalls:sys_enter_mprotect]: 65
@[tracepoint:syscalls:sys_enter_rt_sigprocmask]: 165
@[tracepoint:syscalls:sys_enter_mmap]: 194
@[tracepoint:syscalls:sys_enter_write]: 250
@[tracepoint:syscalls:sys_enter_epoll_pwait]: 560
@[tracepoint:syscalls:sys_enter_rt_sigaction]: 565
@[tracepoint:syscalls:sys_enter_nanosleep]: 632
@[tracepoint:syscalls:sys_enter_futex]: 953
```

udp:
```
@[tracepoint:syscalls:sys_enter_set_tid_address]: 5
@[tracepoint:syscalls:sys_enter_epoll_create1]: 5
@[tracepoint:syscalls:sys_enter_exit_group]: 5
@[tracepoint:syscalls:sys_enter_socket]: 5
@[tracepoint:syscalls:sys_enter_setsockopt]: 5
@[tracepoint:syscalls:sys_enter_getpeername]: 5
@[tracepoint:syscalls:sys_enter_getsockname]: 5
@[tracepoint:syscalls:sys_enter_getrandom]: 5
@[tracepoint:syscalls:sys_enter_connect]: 5
@[tracepoint:syscalls:sys_enter_faccessat]: 5
@[tracepoint:syscalls:sys_enter_eventfd2]: 5
@[tracepoint:syscalls:sys_enter_sched_getaffinity]: 10
@[tracepoint:syscalls:sys_enter_madvise]: 10
@[tracepoint:syscalls:sys_enter_sched_yield]: 12
@[tracepoint:syscalls:sys_enter_brk]: 15
@[tracepoint:syscalls:sys_enter_newfstat]: 15
@[tracepoint:syscalls:sys_enter_epoll_ctl]: 15
@[tracepoint:syscalls:sys_enter_prlimit64]: 20
@[tracepoint:syscalls:sys_enter_tgkill]: 22
@[tracepoint:syscalls:sys_enter_rt_sigreturn]: 22
@[tracepoint:syscalls:sys_enter_getpid]: 22
@[tracepoint:syscalls:sys_enter_clone3]: 24
@[tracepoint:syscalls:sys_enter_gettid]: 29
@[tracepoint:syscalls:sys_enter_set_robust_list]: 29
@[tracepoint:syscalls:sys_enter_rseq]: 29
@[tracepoint:syscalls:sys_enter_fcntl]: 30
@[tracepoint:syscalls:sys_enter_close]: 35
@[tracepoint:syscalls:sys_enter_openat]: 35
@[tracepoint:syscalls:sys_enter_munmap]: 38
@[tracepoint:syscalls:sys_enter_read]: 40
@[tracepoint:syscalls:sys_enter_sigaltstack]: 58
@[tracepoint:syscalls:sys_enter_mprotect]: 63
@[tracepoint:syscalls:sys_enter_rt_sigprocmask]: 159
@[tracepoint:syscalls:sys_enter_mmap]: 192
@[tracepoint:syscalls:sys_enter_write]: 250
@[tracepoint:syscalls:sys_enter_epoll_pwait]: 564
@[tracepoint:syscalls:sys_enter_rt_sigaction]: 565
@[tracepoint:syscalls:sys_enter_nanosleep]: 621
@[tracepoint:syscalls:sys_enter_futex]: 922
```

fifo:
```
@[tracepoint:syscalls:sys_enter_set_tid_address]: 5
@[tracepoint:syscalls:sys_enter_exit_group]: 5
@[tracepoint:syscalls:sys_enter_eventfd2]: 5
@[tracepoint:syscalls:sys_enter_faccessat]: 5
@[tracepoint:syscalls:sys_enter_epoll_create1]: 5
@[tracepoint:syscalls:sys_enter_getrandom]: 5
@[tracepoint:syscalls:sys_enter_sched_yield]: 7
@[tracepoint:syscalls:sys_enter_sched_getaffinity]: 10
@[tracepoint:syscalls:sys_enter_madvise]: 10
@[tracepoint:syscalls:sys_enter_epoll_ctl]: 15
@[tracepoint:syscalls:sys_enter_newfstat]: 15
@[tracepoint:syscalls:sys_enter_brk]: 15
@[tracepoint:syscalls:sys_enter_prlimit64]: 20
@[tracepoint:syscalls:sys_enter_tgkill]: 22
@[tracepoint:syscalls:sys_enter_rt_sigreturn]: 22
@[tracepoint:syscalls:sys_enter_getpid]: 23
@[tracepoint:syscalls:sys_enter_clone3]: 25
@[tracepoint:syscalls:sys_enter_set_robust_list]: 30
@[tracepoint:syscalls:sys_enter_rseq]: 30
@[tracepoint:syscalls:sys_enter_gettid]: 30
@[tracepoint:syscalls:sys_enter_close]: 35
@[tracepoint:syscalls:sys_enter_munmap]: 35
@[tracepoint:syscalls:sys_enter_read]: 40
@[tracepoint:syscalls:sys_enter_openat]: 40
@[tracepoint:syscalls:sys_enter_fcntl]: 40
@[tracepoint:syscalls:sys_enter_sigaltstack]: 60
@[tracepoint:syscalls:sys_enter_mprotect]: 65
@[tracepoint:syscalls:sys_enter_rt_sigprocmask]: 165
@[tracepoint:syscalls:sys_enter_mmap]: 195
@[tracepoint:syscalls:sys_enter_write]: 250
@[tracepoint:syscalls:sys_enter_epoll_pwait]: 296
@[tracepoint:syscalls:sys_enter_nanosleep]: 548
@[tracepoint:syscalls:sys_enter_rt_sigaction]: 565
@[tracepoint:syscalls:sys_enter_futex]: 960
```

That is a lot, but there are a few things we can observe:
- The producers' behaviours (from this perspective) are mostly similar and most of the syscalls are generated by the Go runtime machinery (scheduling, synchronisation and event loop integration)
- We can see the socket related syscalls in all but the fifo implementation, with the additional `setsockopt` calls when the network stack is involved (tcp and udp), but not with UNIX sockets (unixsock and unixgram)
- The fifo implementation stands out a bit here instead of having seemingly lower overhead: no `socket` calls overhead, lower `epoll_pwait`, but we have additional `openat` and `fcntl` calls. It should be interesting when we get to timing comparisons. But recall that the fifo implementation is not flexible enough for our use case out of the box.

