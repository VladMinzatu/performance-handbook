# Linux tracing and performance infrastructure

## The Linux Performance Events subsystem (perf_event_open)

Linux provides a unified kernel instrumentation framework built from a few key subsystems. And both `perf`, as well as `eBPF` (which we'll see later) use this foundation, but in different ways.

The core primitives are:
- The `perf_event_open()` API: a syscall interface to the perf_events subsystem (hardware counters, software counters, tracepoints, sampling). The `perf` tool, `eBPF` perf ringbuffers, as well as any profiler using perf_events will use this.
- kprobes/uprobes: Dynamic instrumentation points that can attach to almost any kernel or user function. Used by both `perf` probes and `eBPF` programs.
- tracepoints: Static instrumentation points defined in kernel code. Again, both `perf` and `eBPF` can attach to these points
- ftrace: Function-level tracer in the kernel, underlying much of perf’s function graph tracing. `perf`, `ftrace`, and `eBPF` (indirectly) make use of this.
- ring buffers (perf buffer or BPF ringbuf): Channels to send data from kernel to user space. Both `perf` and `eBPF` use similar concepts; but `eBPF` adds the BPF-specific ringbufs.
- BPF maps / programs: In-kernel programs with state and logic (filters, aggregations). This concept is unique to `eBPF`.

So both `perf` and `eBPF` share this “perf_event” and tracing substrate. The difference is in how they are used:
- `perf` is essentially a user-space client of the kernel’s perf_events subsystem. It sets up event sources (counters, sampling, probes), collects raw samples, and postprocesses them.
- `eBPF` on the other hand, loads small BPF programs that can run custom logic at each event, instead of the kernel emitting raw samples. Aggregation is done in-kernel, hence less data movement and less overhead.

That difference makes `eBPF` suitable for always-on, safe, low-overhead continuous observability and even profiling. Whereas `perf` is often used for short bursts on test or maybe live systems, but can spike on high sample frequency or many events (context switches, buffer writes, stack unwinds). Generally, "perf is higher overhead and not suitable for production."