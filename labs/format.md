Idea: organize labs around the systems performance debugging, tuning and understanding loop:
- Form a hypothesis based upon our current understanding
- Gather evidence meant to prove or disprove the hypothesis
- Analyze data and update understanding
- Repeat until system is sufficiently understood / well performing.

Tooling: experiments run in Docker (via OrbStack's Linux VM) using the
reusable infrastructure in [tools/](./tools/README.md) - a long-running
privileged "analysis" container (bpftrace, Inspektor Gadget, profilers,
benchmarking tools) plus a per-experiment compose file for the system under
test.

Backlog:
- CPU
- Scheduling
- Memory
- Storage
- Networking and protocols
  - connection pooling in different runtimes
  - http2/3 improvements
- Concurrency & Synchronization
- Go
  - GOMAXPROCS vs. container CPU limits.
  - Netpoller collapsing goroutines onto epoll. 
  - Blocking syscalls vs. network I/O — different thread-growth behavior. (not all blocking is equal) 
  - atomic/lock contention
  - lock vs channel scheduling/coordination overhead
  - go timers and resource/goroutine + missed tick while blocked
  - go backpressure & admission control
  - Goroutine-per-connection scaling ceiling. 
  - Context cancellation leaks in request handling. 
  - Client-side connection pooling and TIME_WAIT churn. (fresh TCP connection per request)
  - futex use in runtime scheduling
  - Nagle's algorithm vs. delayed ACK. 
  - GC pause impact and GOGC/GOMEMLIMIT tuning
  - Escape analysis and hidden heap allocations
  - Mutex contention vs. channels, at the futex level
  - go simd (see 1.27)
- Python
  - async and event loop
  - multiprocessing
- Rust
  - async await and libraries (tokio, async-std)
  - data sharing and synchronization
  - channels for communication (vs Go)
- Virtualization
- Containers & cgroup
- Databases
- Language runtimes and GC
- GPUs / accelerators
  - vLLM tracing and optimization
- Compilers
- NUMA
- Filesystems
- Distributed systems