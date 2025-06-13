# High-Level System Metric Tools

These tools provide a broad overview of system (i.e. host/node/OS) performance by monitoring CPU, memory, disk I/O, and network activity. They're ideal for quickly identifying bottlenecks or misbehaving processes.

## top

Display real-time summary of system resource usage.

Key Features:

- Shows CPU usage, memory usage, load average, and running processes.
- Allows sorting processes by usage.
- Interactive (e.g., press P for CPU sort, M for memory).

Use Case: Quick snapshot of overall system health and resource consumption.

Example Usage:

```
top
```

## htop

An improved, interactive version of top with a user-friendly interface.

Key Features:

- Colorful, scrollable process list.
- Tree view of process hierarchy.
- Supports mouse and keyboard navigation.

Use Case: Visual overview of system metrics with easier navigation and filtering.

Example Usage:

```
htop
```

## iotop

Monitor real-time disk I/O by processes.

Key Features:

- Shows which processes are consuming I/O bandwidth.
- Requires root privileges to display all data.
- Useful for tracking down disk-intensive workloads.

Use Case: Identifying processes causing high disk I/O latency or throughput.

Example Usage:

```
sudo iotop
```

## vmstat

Report virtual memory statistics, CPU activity, and system I/O.

Key Features:

- Lightweight and script-friendly.
- Displays memory swapping, system interrupts, and context switches.
- Useful for time-based sampling.

Use Case: Baseline system performance monitoring over time.

Example Usage:

```
vmstat 1
```

## netstat (deprecated, see ss)

Display network connections, routing tables, and interface stats.

Key Features:

- Reports on open ports and network connections.
- Useful for debugging network issues.
- Largely replaced by ss in newer systems.

Use Case: Checking listening ports, active connections, or diagnosing network problems.

Example Usage:

```
netstat -tulnp
```

## ss (modern alternative to netstat)

Dump socket statistics (TCP/UDP connections, listening ports).

Key Features:

- Faster and more detailed than netstat.
- Supports IPv6, UNIX sockets, and more.

Use Case: Network connection analysis with modern kernel support.

Example Usage:

```
ss -tulnp
```
