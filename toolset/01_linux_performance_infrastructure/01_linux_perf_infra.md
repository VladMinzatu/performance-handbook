# The Linux Performance Events subsystem (perf_event_open)

Linux provides a unified kernel instrumentation framework built from a few key subsystems. And both `perf`, as well as `eBPF` use this foundation, but in different ways.

The core primitives are:

- The `perf_event_open()` API: a syscall interface to the perf_events subsystem (hardware counters, software counters, tracepoints, sampling). The `perf` tool, `eBPF` perf ringbuffers, as well as any profiler using perf_events will use this.
- kprobes/uprobes: Dynamic instrumentation points that can attach to almost any kernel or user function. Used by both `perf` probes and `eBPF` programs.
- tracepoints: Static instrumentation points defined in kernel code. Again, both `perf` and `eBPF` can attach to these points. The available tracepoints can be queried at `/sys/kernel/tracing/available_events`.
- ftrace: Function-level tracer in the kernel, underlying much of perf’s function graph tracing. `perf`, `ftrace`, and `eBPF` (indirectly) make use of this.
- fentry/fexit: Function entry/exit. Replacement for kprobe/kretprobe (and preferred now, available in newer kernel versions) with lower overhead, type-safe function instrumentation via BTF. These are `eBPF` specific attachment points.
- ring buffers (perf buffer or BPF ringbuf): Channels to send data from kernel to user space. Both `perf` and `eBPF` use similar concepts; but `eBPF` adds the BPF-specific ringbufs.
- BPF maps / programs: In-kernel programs with state and logic (filters, aggregations). This concept is unique to `eBPF`.

So both `perf` and `eBPF` share this “perf_event” and tracing substrate. The main difference is in how they are used (and eBPF does add some new capabilities with program types that are not reliant on perf):

- `perf` is essentially a user-space client of the kernel’s perf_events subsystem. It sets up event sources (counters, sampling, probes), collects raw samples, and postprocesses them.
- `eBPF` on the other hand, loads small BPF programs that can run custom logic at each event, instead of the kernel emitting raw samples. Aggregation is done in-kernel, hence less data movement and less overhead.

More specifically, the difference in performance comes from the fact that when we send every raw sample to user space, the user space process is notified (e.g. via epoll) and this causes context switching more frequently (and presumably, some user space processing is done with every sample, but this can be small if aggregation is handled carefully in user space - but then that takes time). With the `eBPF` version, the kernel will update a map (cheap in-kernel memory update) and userspace pulls the aggreageted counts just as often as it needs and then does its postprocessing on it. Additionally, there is kernel -> user memory copying being done when reading in-kernel structures like maps, so the reduced and better bounded amount of data crossing the boundary is also a significant boost (but for ring buffers and perf buffers you can sometimes use `mmap` to avoid the extra copy). Additionally, the bpf syscalls involved also add some overhead, so even when using in-kernel aggregation, we need to take care to use batch APIs to read the maps in user space.

That difference makes `eBPF` suitable for always-on, safe, low-overhead continuous observability and even profiling. Whereas `perf` is often used for short bursts on test or maybe live systems, but can spike on high sample frequency or many events (context switches, buffer writes, stack unwinds). Generally, "perf is higher overhead and not suitable for production." (with eBPF, if done right, <1% CPU overhead is expected, whereas with perf it can be 5-10% or often more - and the difference is primarily due to reduced sample traffic and less work in user space; the perf_event systems is the same, not made inherently faster by eBPF).
