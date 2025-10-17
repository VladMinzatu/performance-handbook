# Linux tracing and performance infrastructure

## The Linux Performance Events subsystem (perf_event_open)

Linux provides a unified kernel instrumentation framework built from a few key subsystems. And both `perf`, as well as `eBPF` (which we'll see later) use this foundation, but in different ways.

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

## Data and visualisation interoperability

We talk about this here as another topic where various tools meet.

While `perf`, `eBPF`, Go’s pprof, and tracing systems all operate at different layers, over the past decade they’ve converged on a few shared output formats and visualization idioms, especially flamegraphs and trace timelines.

### flamegraphs - the universal language for CPU profiles

Flamegraphs are a convention for visualiseing stack-aggregated samples as an inverted icicle chart.
Each box is a function and the width of the box represents its number of samples, while the vertical depth is the call depth.

The input is a text file of stack traces + sample counts:
```
main;foo;bar 10
main;baz 5
```

Uses:
- `perf` is the canonical use case: `perf script | stackcollapse-perf.pl | flamegraph.pl > perf.svg`
- `go tool pprof` supports a flamegraph output
- `eBPF` profilers like `bcc` or `parca-agent` typically export stacks compatible with the flamegraph scripts or the “folded stack” format.

### the pprof profile format

Protocol Buffers schema defined by Google (profile.proto) that is increasingly the common data format for profile storage and visualization. It has the advantages of efficient binary encoding and rich metadata support.

It is used by `go tool pprof -http=:8080` and some continuour profilers.

### trace timelines

They are a visual layer for events rather than stacks. When you’re not sampling stacks but recording timed spans/events (like tracing):

Chrome Trace Format (CTF) and Perfetto trace format have become the cross-domain standard for timeline-style tracing — you can view traces from browsers, Android, kernel, or eBPF tools in one viewer.
