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

Scenario backlog (preferred: a realistic symptom -> investigation with the toolset, ig/bpftrace where possible):
- Everyday incidents
  - "Service is slow, which code?" - baseline CPU profiling workflow: flame graph -> hot path (JSON/regex/logging) -> fix -> diff profile (perf, pprof, py-spy, Pyroscope)
  - "p99 bad at 40% avg CPU" - CFS throttling from container CPU limits, runtime-agnostic (cpu.stat, runqlat)
  - "Latency explodes as traffic grows" - queueing and saturation, knee of the curve, Little's law / USE (wrk, pidstat, run/accept queue depth)
  - "Timeouts when a downstream is slow" - client pool / worker pool exhaustion, timeouts + retries amplifying (ig trace_tcp, ss, bpftrace connect/read latency)
  - "Too many open files" / "cannot assign requested address" - fd leak, ephemeral port exhaustion (lsof, /proc/pid/fd, ss -s, ig trace_open/trace_tcp)
  - "Memory grows until the pod restarts" - real leak (unbounded cache/map/queue), heap profile diffs over time + cgroup memory.stat
  - "The DB is slow" (but it isn't) - time actually in connection acquire / round trips / row decoding (pg_stat_statements vs client-side socket latency)
  - "Disk pegged during log bursts" - excessive/sync logging, log agent competing for I/O (iostat, ig top_block_io, bpftrace write latency)
  - "Slow after deploy/restart" - cold caches and warmup (blks_hit, cachestat, pool warmup)
  - "Everything slower after adding retries" - retry storms, amplification factor, backoff + jitter, timeout budgets
- Python
  - "Every request got slow at once" - asyncio loop blocked by sync/CPU work (py-spy dump, asyncio debug, bpftrace loop-stall detector via epoll_wait gaps)
  - "CPU at 30% but throughput capped" - GIL contention in a threaded service (py-spy --gil, perf with -X perf, futex/runqlat per thread; threads vs processes vs free-threaded)
  - "Workers' RSS grows after fork" - refcount-induced copy-on-write in preforking servers (page faults via bpftrace, smaps_rollup PSS, gc.freeze)
  - "Memory never comes back down" - fragmentation vs leak (memray/tracemalloc vs RSS, pymalloc arenas vs glibc, malloc_trim)
- Binaries, ELF, debugging
  - "Prod flame graph is all hex addresses" - stripped binaries: ELF sections, build IDs, separate .debug files, addr2line, debuginfod
  - "Flame graph stacks are broken/truncated" - frame pointers vs DWARF vs LBR unwinding (perf --call-graph, bpftrace ustack)
  - "Deploy made endpoint 25% slower" - regression hunt down to assembly: lost inline, bounds checks, interface dispatch (perf annotate, objdump, -gcflags=-m)
  - "Process is hung, not crashed" - gdb attach, gcore, py-spy dump, goroutine dumps, off-CPU stacks; core dumps from containers (core_pattern)
- Go
  - "Latency spikes + TIME_WAIT pile-up" - HTTP client body not drained/closed -> connection churn (ig trace_tcp, ss -s, tcp:tcp_set_state)
  - "CPU pegged near memory limit" - GOMEMLIMIT GC death spiral (gctrace, runtime/metrics, memory.events, continuous profiling)
  - "Goroutine count creeps up for days" - context/ticker leak found via goroutine profile diffs in continuous profiling
- Postgres
  - "Queries degrade over a week" - idle-in-transaction holding xmin horizon, vacuum can't clean, bloat (pg_stat_activity, pgstattuple)
  - "p99 spikes every few minutes" - checkpoint/WAL fsync storms (pg_stat_checkpointer, biolatency, ig top_block_io)
  - "Report query slow, disk busy" - work_mem spill to temp files (log_temp_files, ig trace_open on pgsql_tmp)
- Containers & Linux
  - "App is slow because of logging" - write() to stdout pipe blocking on a slow log consumer (bpftrace write latency)
  - "Noisy neighbour" - PSI (cpu/memory/io.pressure) as the continuous signal, then attribution (ig top_block_io, runqlat)

Topic backlog:
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
  - go ringbuf for SPSC improvement
  - Go 1.27 portable SIMD (asm vs Python numpy) 
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
  - io_uring performance improvement (a la PG)
- Python
  - async and event loop
  - multiprocessing
- Rust
  - async await and libraries (tokio, async-std)
  - data sharing and synchronization
  - channels for communication (vs Go)
- Virtualization
- Containers & cgroup
- Databases (https://www.interdb.jp/pg/)
  - postgres iouring
- Language runtimes and GC
- Assembly optimizations
- GPUs / accelerators
  - vLLM tracing and optimization
- Compilers
- NUMA
- Filesystems
- Distributed systems