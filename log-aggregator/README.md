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

### syscall counting

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

### syscall latencies

Next, we will check the latencies of some of the syscalls that we expect would be more interesting, by running the following bpftrace custom tool while we run our test experiments:
```
sudo ./syscall_latency.bt
```

And here are the outputs for the different IPC types:

unixsock:
```
@lat[write]:
[1K, 2K)               2 |@                                                   |
[2K, 4K)               9 |@@@@@@                                              |
[4K, 8K)              35 |@@@@@@@@@@@@@@@@@@@@@@@@                            |
[8K, 16K)             65 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@       |
[16K, 32K)            74 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[32K, 64K)            44 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                      |
[64K, 128K)           13 |@@@@@@@@@                                           |
[128K, 256K)           8 |@@@@@                                               |
[256K, 512K)           7 |@@@@                                                |
[512K, 1M)             2 |@                                                   |

@lat[epoll_pwait]:
[512, 1K)              1 |                                                    |
[1K, 2K)              11 |@@                                                  |
[2K, 4K)              11 |@@                                                  |
[4K, 8K)               8 |@                                                   |
[8K, 16K)              6 |@                                                   |
[16K, 32K)             4 |                                                    |
[32K, 64K)            15 |@@@                                                 |
[64K, 128K)           21 |@@@@                                                |
[128K, 256K)          49 |@@@@@@@@@@                                          |
[256K, 512K)          63 |@@@@@@@@@@@@@                                       |
[512K, 1M)            63 |@@@@@@@@@@@@@                                       |
[1M, 2M)              48 |@@@@@@@@@                                           |
[2M, 4M)               0 |                                                    |
[4M, 8M)               0 |                                                    |
[8M, 16M)              0 |                                                    |
[16M, 32M)             0 |                                                    |
[32M, 64M)             0 |                                                    |
[64M, 128M)          250 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|

@lat[futex]:
[512, 1K)              5 |                                                    |
[1K, 2K)              17 |@                                                   |
[2K, 4K)              72 |@@@@@@@                                             |
[4K, 8K)              77 |@@@@@@@                                             |
[8K, 16K)            138 |@@@@@@@@@@@@@@                                      |
[16K, 32K)            55 |@@@@@                                               |
[32K, 64K)            21 |@@                                                  |
[64K, 128K)           13 |@                                                   |
[128K, 256K)          15 |@                                                   |
[256K, 512K)           5 |                                                    |
[512K, 1M)             5 |                                                    |
[1M, 2M)               2 |                                                    |
[2M, 4M)               1 |                                                    |
[4M, 8M)               0 |                                                    |
[8M, 16M)              0 |                                                    |
[16M, 32M)             0 |                                                    |
[32M, 64M)             0 |                                                    |
[64M, 128M)          508 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[128M, 256M)           4 |                                                    |
[256M, 512M)          10 |@                                                   |
[512M, 1G)            10 |@                                                   |
[1G, 2G)               5 |                                                    |
[2G, 4G)               2 |                                                    |
[4G, 8G)              10 |@                                                   |
```

tcp:
```
@lat[write]:
[4K, 8K)               1 |                                                    |
[8K, 16K)              7 |@@@                                                 |
[16K, 32K)            49 |@@@@@@@@@@@@@@@@@@@@@@@@@@                          |
[32K, 64K)            98 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[64K, 128K)           73 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@              |
[128K, 256K)          17 |@@@@@@@@@                                           |
[256K, 512K)           5 |@@                                                  |

@lat[epoll_pwait]:
[1K, 2K)               2 |                                                    |
[2K, 4K)               5 |@                                                   |
[4K, 8K)               1 |                                                    |
[8K, 16K)              0 |                                                    |
[16K, 32K)             1 |                                                    |
[32K, 64K)             0 |                                                    |
[64K, 128K)            1 |                                                    |
[128K, 256K)           0 |                                                    |
[256K, 512K)           1 |                                                    |
[512K, 1M)             1 |                                                    |
[1M, 2M)              73 |@@@@@@@@@@@@@@@                                     |
[2M, 4M)               0 |                                                    |
[4M, 8M)               0 |                                                    |
[8M, 16M)              0 |                                                    |
[16M, 32M)             0 |                                                    |
[32M, 64M)             0 |                                                    |
[64M, 128M)          250 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|

@lat[futex]:
[512, 1K)              5 |                                                    |
[1K, 2K)              17 |@                                                   |
[2K, 4K)              29 |@@@                                                 |
[4K, 8K)              72 |@@@@@@@                                             |
[8K, 16K)            117 |@@@@@@@@@@@@                                        |
[16K, 32K)            74 |@@@@@@@                                             |
[32K, 64K)            36 |@@@                                                 |
[64K, 128K)           16 |@                                                   |
[128K, 256K)           6 |                                                    |
[256K, 512K)           6 |                                                    |
[512K, 1M)             5 |                                                    |
[1M, 2M)               1 |                                                    |
[2M, 4M)               0 |                                                    |
[4M, 8M)               0 |                                                    |
[8M, 16M)              0 |                                                    |
[16M, 32M)             0 |                                                    |
[32M, 64M)             0 |                                                    |
[64M, 128M)          501 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[128M, 256M)           3 |                                                    |
[256M, 512M)           4 |                                                    |
[512M, 1G)             3 |                                                    |
[1G, 2G)               6 |                                                    |
[2G, 4G)               4 |                                                    |
[4G, 8G)               7 |                                                    |
```

unixgram:
```
@lat[write]:
[2K, 4K)               1 |                                                    |
[4K, 8K)              23 |@@@@@@@@@@@@@                                       |
[8K, 16K)             57 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                   |
[16K, 32K)            88 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[32K, 64K)            50 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                       |
[64K, 128K)           15 |@@@@@@@@                                            |
[128K, 256K)           7 |@@@@                                                |
[256K, 512K)           6 |@@@                                                 |
[512K, 1M)             3 |@                                                   |

@lat[epoll_pwait]:
[1K, 2K)               3 |                                                    |
[2K, 4K)              23 |@@@@                                                |
[4K, 8K)              22 |@@@@                                                |
[8K, 16K)              3 |                                                    |
[16K, 32K)             1 |                                                    |
[32K, 64K)             2 |                                                    |
[64K, 128K)           23 |@@@@                                                |
[128K, 256K)          73 |@@@@@@@@@@@@@@@                                     |
[256K, 512K)          54 |@@@@@@@@@@@                                         |
[512K, 1M)            40 |@@@@@@@@                                            |
[1M, 2M)              84 |@@@@@@@@@@@@@@@@@                                   |
[2M, 4M)               1 |                                                    |
[4M, 8M)               0 |                                                    |
[8M, 16M)              0 |                                                    |
[16M, 32M)             0 |                                                    |
[32M, 64M)             0 |                                                    |
[64M, 128M)          250 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|

@lat[futex]:
[256, 512)             3 |                                                    |
[512, 1K)              2 |                                                    |
[1K, 2K)              15 |@                                                   |
[2K, 4K)              46 |@@@@                                                |
[4K, 8K)              70 |@@@@@@@                                             |
[8K, 16K)            131 |@@@@@@@@@@@@@                                       |
[16K, 32K)            61 |@@@@@@                                              |
[32K, 64K)            27 |@@                                                  |
[64K, 128K)           16 |@                                                   |
[128K, 256K)          15 |@                                                   |
[256K, 512K)           9 |                                                    |
[512K, 1M)             4 |                                                    |
[1M, 2M)               0 |                                                    |
[2M, 4M)               0 |                                                    |
[4M, 8M)               0 |                                                    |
[8M, 16M)              0 |                                                    |
[16M, 32M)             0 |                                                    |
[32M, 64M)             0 |                                                    |
[64M, 128M)          505 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[128M, 256M)           0 |                                                    |
[256M, 512M)           2 |                                                    |
[512M, 1G)             4 |                                                    |
[1G, 2G)               6 |                                                    |
[2G, 4G)               1 |                                                    |
[4G, 8G)               9 |                                                    |
```

udp:
```
@lat[write]:
[4K, 8K)               8 |@@@@                                                |
[8K, 16K)             28 |@@@@@@@@@@@@@@@@                                    |
[16K, 32K)            90 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[32K, 64K)            60 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                  |
[64K, 128K)           54 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                     |
[128K, 256K)          10 |@@@@@                                               |
[256K, 512K)          10 |@@@@@                                               |
[512K, 1M)             2 |@                                                   |

@lat[epoll_pwait]:
[512, 1K)             10 |@@                                                  |
[1K, 2K)              38 |@@@@@@@                                             |
[2K, 4K)              91 |@@@@@@@@@@@@@@@@@@                                  |
[4K, 8K)              64 |@@@@@@@@@@@@@                                       |
[8K, 16K)              9 |@                                                   |
[16K, 32K)             1 |                                                    |
[32K, 64K)             6 |@                                                   |
[64K, 128K)           12 |@@                                                  |
[128K, 256K)          13 |@@                                                  |
[256K, 512K)          11 |@@                                                  |
[512K, 1M)             8 |@                                                   |
[1M, 2M)              88 |@@@@@@@@@@@@@@@@@@                                  |
[2M, 4M)               3 |                                                    |
[4M, 8M)               0 |                                                    |
[8M, 16M)              0 |                                                    |
[16M, 32M)             0 |                                                    |
[32M, 64M)             0 |                                                    |
[64M, 128M)          250 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|

@lat[futex]:
[512, 1K)              3 |                                                    |
[1K, 2K)              12 |@                                                   |
[2K, 4K)              47 |@@@@                                                |
[4K, 8K)              52 |@@@@@                                               |
[8K, 16K)            120 |@@@@@@@@@@@@                                        |
[16K, 32K)            59 |@@@@@@                                              |
[32K, 64K)            56 |@@@@@                                               |
[64K, 128K)           17 |@                                                   |
[128K, 256K)          11 |@                                                   |
[256K, 512K)          10 |@                                                   |
[512K, 1M)             1 |                                                    |
[1M, 2M)               8 |                                                    |
[2M, 4M)               3 |                                                    |
[4M, 8M)               1 |                                                    |
[8M, 16M)              0 |                                                    |
[16M, 32M)             0 |                                                    |
[32M, 64M)             0 |                                                    |
[64M, 128M)          504 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[128M, 256M)           1 |                                                    |
[256M, 512M)           3 |                                                    |
[512M, 1G)             3 |                                                    |
[1G, 2G)               4 |                                                    |
[2G, 4G)               5 |                                                    |
[4G, 8G)              10 |@                                                   |

```

fifo:
```
@lat[write]:
[4K, 8K)              15 |@@@@@@@@@@                                          |
[8K, 16K)             63 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@       |
[16K, 32K)            72 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[32K, 64K)            46 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                   |
[64K, 128K)           34 |@@@@@@@@@@@@@@@@@@@@@@@@                            |
[128K, 256K)          13 |@@@@@@@@@                                           |
[256K, 512K)           5 |@@@                                                 |
[512K, 1M)             2 |@                                                   |

@lat[epoll_pwait]:
[1K, 2K)               3 |                                                    |
[2K, 4K)               2 |                                                    |
[4K, 8K)               0 |                                                    |
[8K, 16K)              0 |                                                    |
[16K, 32K)             0 |                                                    |
[32K, 64K)             0 |                                                    |
[64K, 128K)            0 |                                                    |
[128K, 256K)           1 |                                                    |
[256K, 512K)           0 |                                                    |
[512K, 1M)             2 |                                                    |
[1M, 2M)             101 |@@@@@@@@@@@@@@@@@@@@@                               |
[2M, 4M)               1 |                                                    |
[4M, 8M)               0 |                                                    |
[8M, 16M)              0 |                                                    |
[16M, 32M)             0 |                                                    |
[32M, 64M)             0 |                                                    |
[64M, 128M)          250 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|

@lat[futex]:
[256, 512)             1 |                                                    |
[512, 1K)              4 |                                                    |
[1K, 2K)              16 |@                                                   |
[2K, 4K)              33 |@@@                                                 |
[4K, 8K)              52 |@@@@@                                               |
[8K, 16K)            108 |@@@@@@@@@@@                                         |
[16K, 32K)            86 |@@@@@@@@                                            |
[32K, 64K)            42 |@@@@                                                |
[64K, 128K)           14 |@                                                   |
[128K, 256K)          10 |@                                                   |
[256K, 512K)           9 |                                                    |
[512K, 1M)             5 |                                                    |
[1M, 2M)               1 |                                                    |
[2M, 4M)               2 |                                                    |
[4M, 8M)               2 |                                                    |
[8M, 16M)              0 |                                                    |
[16M, 32M)             0 |                                                    |
[32M, 64M)             0 |                                                    |
[64M, 128M)          502 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[128M, 256M)           2 |                                                    |
[256M, 512M)           3 |                                                    |
[512M, 1G)             3 |                                                    |
[1G, 2G)               4 |                                                    |
[2G, 4G)               6 |                                                    |
[4G, 8G)               9 |                                                    |
```

Let's try to interpret what we are seeing here:
- first, the similar distributions of `futex` and `epoll_pwait` for each IPC type clearly show how our pattern of log generation during the test interact with the system through the Go runtime: we generate the messages at 100ms intervals with no jitter. `futex` is used under the hood for the Ticker: ticker fires -> goroutine wakes up via `futex`. And the writes are handled through the netpoller, with system calls blocking the goroutine (but not the underlying thread) for the 100ms between messages.
- the performance of the `write`s themselves is not very different across IPC types - we can argue that the overhead in the tcp case is noticeable as it skews the distribution a bit. Fifo seems to be the fastest perhaps, not surprisingly given that it's the simples IPC method (kernel buffers without networking stack).

### wakeup-to-running latencies

Next, we look at the latencies between aggregator wakeup to aggregator being scheduled:
```
sudo ./aggregator_sched_latency.bt
```
We get the following results:

fifo:
```
@lat:
[4K, 8K)               1 |@                                                   |
[8K, 16K)             12 |@@@@@@@@@@@@@@@@@@@@@                               |
[16K, 32K)            27 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@    |
[32K, 64K)            29 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[64K, 128K)           15 |@@@@@@@@@@@@@@@@@@@@@@@@@@                          |
[128K, 256K)           8 |@@@@@@@@@@@@@@                                      |
[256K, 512K)           0 |                                                    |
[512K, 1M)             3 |@@@@@                                               |
```

