# Dynamic Tracing & eBPF-based Tools

Dynamic tracing tools use in-kernel instrumentation to gather rich runtime data with low overhead. eBPF (extended Berkeley Packet Filter) enables safe and efficient programs to run in kernel space, empowering tools for tracing, performance analysis, observability, and security.

> **Overhead**: The technologies in this category are generally production safe when used right: eBPF can power production agents (developed with CO-RE) and tools like BCC and bpftrace are tools that can be used for ad-hoc investigations in a production-safe way in terms of overhead.

## [eBPF](https://ebpf.io/) (not a tool - the infra underlying all the tools below)

General-purpose in-kernel virtual machine for running sandboxed programs.

Key Features:

- Can attach to tracepoints, kprobes, uprobes, and network events.
- Runs securely in the kernel without needing kernel modules.
- Forms the foundation of modern observability tools (bpftrace, BCC, etc.).

Use Case: Framework for high-performance, low-overhead kernel instrumentation.

> Read the great book "Learning eBPF" by Liz Rice.

## [bpftrace](https://github.com/bpftrace/bpftrace)

High-level, user-friendly tracing tool based on eBPF.

Key Features:

- Scripting language similar to awk or DTrace.
- One-liners for powerful system observability.
- Can attach to kprobes, uprobes, tracepoints, etc.

Use Case: Rapid prototyping and exploratory debugging of kernel/user events.

Example usage:

```
bpftrace -e 'tracepoint:syscalls:sys_enter_execve { printf("%s\n", comm); }'
```

## [BCC](https://github.com/iovisor/bcc) (BPF Compiler Collection)

Toolkit and Python/C++ framework for writing advanced eBPF tools.

Key Features:

- Dozens of ready-to-use tools (like execsnoop, tcpconnect, biosnoop).
- Allows custom tool development with Python/C.
- More powerful but more complex than bpftrace.

Use Case: In-depth tracing tools for performance, networking, and security.

Example usage:

```
sudo /usr/share/bcc/tools/tcpconnect
```

## tracee

Runtime security and observability tool using eBPF.

Key Features:

- Focused on detecting suspicious behavior via syscall/event tracing.
- Container-aware.
- Built by Aqua Security (open-source).

Use Case: eBPF-based runtime threat detection and compliance monitoring.

Example usage:

```
sudo ./tracee --trace event=execve
```

## flamegraph

Visualize stack traces as flame graphs to identify performance bottlenecks.

Key Features:

- Works with perf, bpftrace, BCC, and other tracers.
- Helps understand which code paths consume CPU time.

Use Case: Post-process stack sampling data to visualize hotspots.

```
perf record -F 99 -a -g -- your_app
perf script | ./stackcollapse-perf.pl | ./flamegraph.pl > flamegraph.svg
```

## Notes

- Tools like `bpftrace` and `BCC` require a kernel with eBPF support (Linux 4.9+ recommended).
- `flamegraph` is not an eBPF tool by itself but integrates well with others for visualization.
- Many `eBPF` tools require root access or appropriate `CAP_BPF` privileges.

Tutorial: https://www.brendangregg.com/ebpf.html
