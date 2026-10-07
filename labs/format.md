Lab format (current):
- One focused takeaway per lab, answering mainly: how to use/tune the
  mechanism in production, and how to diagnose it when it goes wrong. A
  topic with several takeaways becomes several labs (e.g. a series on Go
  HTTP client connection pools), not one lab with many predictions.
- Topic-oriented: a mechanism with a clear connection to the kinds of real
  systems that rely on it, pushed to where it reaches its limits or fails.
- Tools most relevant in production, prioritizing `ig`, `bpftrace` and
  `perf`.
- Each lab is self-contained: no references to other labs, and no
  narration of sandbox/tooling backstory.
- README structure (reference: [0108](./0108-go-http-client-idle-pool/README.md)):
  - `# <Mechanism>: <takeaway>` title, then one line on how the lab uses
    the shared [tools/](./tools/README.md) infrastructure.
  - `## Background` - the mechanism, its defaults, the systems that hit
    it, what it costs, and how it shows up from the outside.
  - `## Hypotheses` - 2-4 bold, numbered predictions, all serving the one
    takeaway, with concrete expected numbers/signals; optionally one
    marked (stretch).
  - `## Setup` - start commands, one bullet per container (what it does,
    its knobs and defaults), then the observation commands the
    experiments use (logs, `ig`, `ss` via `nsenter`, cgroup lookup for
    `bpftrace` filters), and how to change knobs.
  - `## Experiments` (link to `./experiments`) and `## Tear down`.
- Experiment files (`experiments/NN_<name>.md`): a `##` title, which
  predictions it tests and why, then commands each followed by
  "producing output:" with real captured output and an interpretation.
- Keep it focused on one setting/configuration: what it does, where its
  limits are, and how to detect when you've hit them.

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
  - HTTP client connection pool series (one takeaway per lab):
    - 0108 idle pool: MaxIdleConnsPerHost default 2 -> connection churn, TIME_WAIT, port exhaustion (ig trace_tcp, ss -s)
    - max conns: MaxConnsPerHost as a queue with a throughput ceiling; size from rate x degraded latency (ss -tin, goroutine dump in getConn)
    - TLS churn under a CPU limit: run-queue latency tells waiting-for-CPU from waiting-on-something-else (bpftrace runqlat, cpu.stat)
    - in-flight memory when the downstream slows: unlimited vs capped pool, neither bounds the client (memory.stat anon/sock, ss -tm)
    - load shedding: a fail-fast limit in front of the pool bounds memory and accepted latency
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

Scenario backlog (on hold - the lab format above is preferred for now; a realistic symptom -> investigation with the toolset, ig/bpftrace where possible):
- Everyday incidents
  - "Service is slow, which code?" - baseline CPU profiling workflow: flame graph -> hot path (JSON/regex/logging) -> fix -> diff profile (perf, pprof, py-spy, Pyroscope)
    - Go CPU profiling loop: pprof flame graph -> per-request hot path (regex compile, JSON reflection) -> fix -> `pprof -diff_base` confirms the frame shrank
    - Python CPU profiling loop: py-spy record/top on a live service vs cProfile - sampling vs instrumenting bias on small hot functions
    - Continuous profiling: Pyroscope comparison of two time ranges to find a regression after the fact
  - "p99 bad at 40% avg CPU" - CFS throttling from container CPU limits, runtime-agnostic (cpu.stat, runqlat)
    - CFS quota throttling: bursty multi-threaded handling exhausts quota early in each period -> nr_throttled/throttled_usec track p99 while avg CPU stays low
    - Period and burst tuning: same budget with different cpu.max period / cpu.max.burst -> tail latency changes, average doesn't
    - Limits vs weights: cpu.weight-only container never throttles, only contends (runqlat vs cpu.stat)
  - "Latency explodes as traffic grows" - queueing and saturation, knee of the curve, Little's law / USE (wrk, pidstat, run/accept queue depth)
    - Knee of the curve: fixed-worker service under an open-loop rate sweep -> latency vs utilization, Little's law (L = λW) checked against measured in-flight
    - Accept queue overflow: listen backlog fills -> SYN drops, 1s/3s retransmit latency steps (ss -lnt Recv-Q, nstat ListenOverflows, ig trace_tcpdrop)
    - Run-queue saturation: more runnable threads than CPUs -> runqlat grows before CPU looks pegged
    - USE method walkthrough: utilization/saturation/errors per resource for one service under rising load
  - "Timeouts when a downstream is slow" - client pool / worker pool exhaustion, timeouts + retries amplifying (ig trace_tcp, ss, bpftrace connect/read latency)
    - Client connection-pool exhaustion: MaxConnsPerHost caps in-flight calls -> latency is pool wait, not downstream time (ss -tin, goroutine dump in getConn)
    - Bulkheads: one shared worker limit lets a slow dependency take down unrelated endpoints; per-dependency limits keep them flat
    - Timeouts destroy pooled connections: timeout mid-response forces close + redial -> connect storm, TIME_WAIT (ig trace_tcp, ss -s, bpftrace connect latency)
    - Cancellation propagation: downstream keeps working on requests the caller abandoned -> wasted CPU, fixed by deadline propagation
    - Closed- vs open-loop load: wrk hides the incident that vegeta/wrk2 exposes at the same nominal rate (coordinated omission)
    - Network vs server slowness: tc netem delay raises ss -ti RTT; server service time doesn't, but both raise request->first-byte latency
  - "Too many open files" / "cannot assign requested address" - fd leak, ephemeral port exhaustion (lsof, /proc/pid/fd, ss -s, ig trace_open/trace_tcp)
    - fd leak: unclosed files/bodies -> /proc/pid/fd grows until EMFILE (ig trace_open, bpftrace open/close balance per cgroup)
    - RLIMIT_NOFILE in containers: soft vs hard limits, Go raises the soft limit itself, Python doesn't
    - Ephemeral port exhaustion: connect-per-request to one destination fills ip_local_port_range with TIME_WAIT -> EADDRNOTAVAIL (ss -s, tcp_tw_reuse)
    - EMFILE in an accept loop: server spins or drops clients under fd pressure
  - "Memory grows until the pod restarts" - real leak (unbounded cache/map/queue), heap profile diffs over time + cgroup memory.stat
    - Unbounded map/cache in Go: inuse_space heap profile diffs over time point at the allocation site; memory.stat anon tracks it
    - Unbounded queue without backpressure: memory follows queue depth under overload, not a leak per se
    - Not a leak: memory.current includes reclaimable page cache (memory.stat file vs anon, cachestat)
    - Python leak: tracemalloc snapshot diffs / memray on a growing global
  - "The DB is slow" (but it isn't) - time actually in connection acquire / round trips / row decoding (pg_stat_statements vs client-side socket latency)
    - Pool acquire wait: client-side timing splits acquire vs query; pg_stat_statements shows fast queries
    - Round-trip multiplier: tc netem adds 1ms and per-request latency scales with round trips per request
    - Client-side decoding: large result sets / ORM hydration dominate client CPU (pprof/py-spy) while server time is small
    - Server vs client view: pg_stat_statements / log_min_duration_statement vs bpftrace send->recv latency in the client cgroup
  - "Disk pegged during log bursts" - excessive/sync logging, log agent competing for I/O (iostat, ig top_block_io, bpftrace write latency)
    - Sync logging: fsync/O_SYNC per line -> request latency = flush latency (bpftrace write/fsync latency, iostat await)
    - Buffered vs unbuffered logging: write() syscalls per request and throughput (bpftrace syscall counts)
    - Dirty page writeback storms: page cache absorbs bursts then stalls writers in balance_dirty_pages (/proc/vmstat, dirty_ratio)
    - Competing log agent: io.weight/io.max on the agent's cgroup restores app latency (ig top_block_io, io.pressure)
  - "Slow after deploy/restart" - cold caches and warmup (blks_hit, cachestat, pool warmup)
    - Cold caches: Postgres restart vs OS cache drop separately -> shared_buffers vs page cache vs disk (blks_hit, cachestat, biolatency); pg_prewarm
    - Pool warmup: first requests pay connect + auth; pre-warmed pool removes the burst (ig trace_tcp)
    - Lazy init: first-N-requests latency from on-demand init (regex/template compile, imports)
  - "Everything slower after adding retries" - retry storms, amplification factor, backoff + jitter, timeout budgets
    - Retry amplification + metastable failure: downstream load ≈ rate × (1 + retries); removing the fault doesn't recover until load drops
    - Synchronized retries: fixed backoff causes thundering-herd waves after an outage; jitter flattens them
    - Retry budgets and timeout budgets: capping retry ratio and sharing one deadline across attempts
- Python
  - "Every request got slow at once" - asyncio loop blocked by sync/CPU work (py-spy dump, asyncio debug, bpftrace loop-stall detector via epoll_wait gaps)
    - Blocking call in the event loop: sync I/O stalls all tasks -> unrelated requests' latency = block duration (asyncio debug slow_callback, py-spy dump)
    - CPU work in the loop: large JSON parsing; offload to executor/process pool and compare
    - Loop-stall detector: bpftrace gaps between epoll_wait calls per thread in the container cgroup
    - Default executor saturation: to_thread/run_in_executor queueing at max_workers
  - "CPU at 30% but throughput capped" - GIL contention in a threaded service (py-spy --gil, perf with -X perf, futex/runqlat per thread; threads vs processes vs free-threaded)
    - Threaded CPU-bound handlers: throughput flat beyond one thread, py-spy --gil shows the holder, futex waits per thread
    - Threads vs processes vs free-threaded build on the same workload
    - GIL-releasing C extensions (hashlib/numpy) scale with threads where pure Python doesn't
    - Convoy effect: CPU-bound thread inflates I/O thread latency; sys.setswitchinterval effect
  - "Workers' RSS grows after fork" - refcount-induced copy-on-write in preforking servers (page faults via bpftrace, smaps_rollup PSS, gc.freeze)
    - Refcount COW: preforking server with preloaded data -> per-worker USS/PSS grows on read-only access (smaps_rollup, bpftrace page faults)
    - gc.freeze before fork: removes GC-induced page writes, measured the same way
  - "Memory never comes back down" - fragmentation vs leak (memray/tracemalloc vs RSS, pymalloc arenas vs glibc, malloc_trim)
    - pymalloc arenas pinned by a few survivors: RSS stays high after freeing (sys._debugmallocstats)
    - glibc per-thread arenas inflating RSS: MALLOC_ARENA_MAX, malloc_trim, jemalloc comparison
    - Leak vs fragmentation: tracemalloc flat while RSS stays high
- Binaries, ELF, debugging
  - "Prod flame graph is all hex addresses" - stripped binaries: ELF sections, build IDs, separate .debug files, addr2line, debuginfod
    - Stripped C binary: ELF sections, build ID, objcopy --only-keep-debug, symbolization via separate debug file
    - Local debuginfod serving debug info by build ID
    - Go -ldflags='-s -w': .symtab/DWARF gone but .gopclntab keeps function names
    - Profiling into containers: resolving binaries via /proc/pid/root from the analysis container
  - "Flame graph stacks are broken/truncated" - frame pointers vs DWARF vs LBR unwinding (perf --call-graph, bpftrace ustack)
    - Frame pointers on/off in C: bpftrace ustack complete vs truncated
    - DWARF unwinding cost/size trade-off (perf --call-graph dwarf; needs a real Linux VM)
    - Mixed stacks: Go cgo/libc frames, Python interpreter vs Python frames (py-spy --native, perf trampolines)
  - "Deploy made endpoint 25% slower" - regression hunt down to assembly: lost inline, bounds checks, interface dispatch (perf annotate, objdump, -gcflags=-m)
    - Lost inlining: function grows past the inline budget -> call overhead in hot loop (-gcflags=-m, objdump)
    - Bounds checks: -d=ssa/check_bce/debug=1 and loop rewrites that eliminate them
    - Interface dispatch vs concrete calls; PGO devirtualization
    - Detecting the regression reliably: benchstat and noise
  - "Process is hung, not crashed" - gdb attach, gcore, py-spy dump, goroutine dumps, off-CPU stacks; core dumps from containers (core_pattern)
    - Go deadlock: SIGQUIT goroutine dump reveals the lock cycle
    - Hung C/Python process: gdb attach, py-spy dump --native, /proc/pid/stack and wchan for D-state waits
    - Off-CPU analysis: bpftrace sched_switch stacks showing where threads sleep
    - Core dumps from containers: host-global core_pattern, gcore of a live process, offline analysis
- Go
  - "Latency spikes + TIME_WAIT pile-up" - HTTP client body not drained/closed -> connection churn (ig trace_tcp, ss -s, tcp:tcp_set_state)
    - Body not drained/closed -> no reuse -> new connection per request (ig trace_tcp connect rate, ss -s)
    - MaxIdleConnsPerHost=2 default: high-concurrency client churns even with correct body handling
    - Keep-alive mismatch: server idle timeout shorter than client's -> resets on reuse (tcp:tcp_set_state, RST counts)
  - "CPU pegged near memory limit" - GOMEMLIMIT GC death spiral (gctrace, runtime/metrics, memory.events, continuous profiling)
    - Live heap near GOMEMLIMIT: GC runs continuously, CPU spikes, throughput collapses (gctrace, runtime/metrics, cpu.stat)
    - GOGC=off + GOMEMLIMIT vs defaults under a container limit: OOM vs death spiral (memory.events)
    - GC CPU limiter: behaviour when the limit can't be met
  - "Goroutine count creeps up for days" - context/ticker leak found via goroutine profile diffs in continuous profiling
    - Goroutines blocked on send after the receiver timed out: goroutine profile diff by stack
    - Unstopped tickers / never-cancelled contexts: slow goroutine + memory growth
    - Detection via continuous goroutine profiles over hours (Pyroscope)
- Postgres
  - "Queries degrade over a week" - idle-in-transaction holding xmin horizon, vacuum can't clean, bloat (pg_stat_activity, pgstattuple)
    - Idle-in-transaction pins xmin: n_dead_tup grows, VACUUM removes nothing (pg_stat_activity backend_xmin, VACUUM VERBOSE)
    - Bloat impact on query latency: pgstattuple, buffers in EXPLAIN (ANALYZE, BUFFERS)
    - Other horizon holders and guardrails: long analytic queries, idle_in_transaction_session_timeout
  - "p99 spikes every few minutes" - checkpoint/WAL fsync storms (pg_stat_checkpointer, biolatency, ig top_block_io)
    - Checkpoint storms: small max_wal_size -> frequent checkpoints, latency spikes aligned with them (log_checkpoints, bpftrace fsync latency)
    - checkpoint_completion_target spreading the write burst
    - Full-page writes after checkpoints inflate WAL volume (pg_stat_wal wal_fpi)
    - synchronous_commit on vs off: commit latency = WAL fsync latency
  - "Report query slow, disk busy" - work_mem spill to temp files (log_temp_files, ig trace_open on pgsql_tmp)
    - Sort spill: EXPLAIN ANALYZE external merge, log_temp_files, ig trace_open on pgsql_tmp
    - Hash aggregate/join spill and hash_mem_multiplier
    - work_mem × nodes × backends: memory risk of raising it globally
- Containers & Linux
  - "App is slow because of logging" - write() to stdout pipe blocking on a slow log consumer (bpftrace write latency)
    - Full stdout pipe: slow consumer -> write() blocks the app (bpftrace write latency on fd 1, /proc/pid/stack in pipe_write)
    - Docker log driver modes: blocking vs non-blocking with max-buffer-size (block vs drop)
    - Async logging buffers: latency vs log loss trade-off
  - "Noisy neighbour" - PSI (cpu/memory/io.pressure) as the continuous signal, then attribution (ig top_block_io, runqlat)
    - CPU neighbour: victim's cpu.pressure rises, runqlat attribution, cpu.weight mitigation
    - Memory neighbour: victim's page cache evicted (memory.pressure, cachestat), memory.low protection
    - I/O neighbour: io.pressure, ig top_block_io attribution, io.weight/io.max mitigation
