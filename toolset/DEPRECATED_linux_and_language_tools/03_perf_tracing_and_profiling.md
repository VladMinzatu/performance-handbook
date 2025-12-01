# perf tracing and profiling

All tools mentioned here are part of most major distributions, though you may need to install them (e.g., via apt, yum, or dnf).

These tools are suitable for test enviornments or brief production usage, as they introduce non negligible overhead.

These tools provide fine-grained details about system behavior — from CPU cycles and function calls to system calls and library usage. They're essential for debugging performance issues, understanding application behavior, and profiling workloads.

## perf

Performance analysis and CPU profiling tool.

Key Features:

- Records CPU performance counters, software events, hardware events, etc.
- Supports sampling and tracing (e.g., perf top, perf record).
- Can profile kernel and user-space code.

Use Case: Identify CPU hotspots and performance bottlenecks in code.

Example Usage:

```
perf top
perf record ./your_app
perf report
```

Tutorials:

- https://perfwiki.github.io/main/
- https://www.brendangregg.com/perf.html

## strace

Trace system calls and signals of a process.

Key Features:

- Intercepts and logs all syscalls made by a process.
- Great for debugging unexpected behavior or I/O issues.
- Supports attaching to running processes.

Use Case: Understand what syscalls a process makes and in what order.

Example Usage:

```
strace ./your_app
strace -p <pid>
```

## ltrace

Trace library calls and signals of a process.

Key Features:

- Intercepts and logs dynamic library calls (like malloc, printf).
- Complements strace by showing higher-level library interactions.

Use Case: See which shared library functions are being used by a process.

Example Usage:

```
ltrace ./your_app
```

## ftrace

Built-in Linux kernel tracer for debugging and profiling.

Key Features:

- Can trace function calls, interrupts, scheduling, and more.
- Very low overhead.
- Accessible via /sys/kernel/debug/tracing/.
- Underpins many other tracing tools.

Use Case: Kernel-level performance tracing and debugging.

Example Usage:

```
echo function > /sys/kernel/debug/tracing/current_tracer
cat /sys/kernel/debug/tracing/trace
```

## Notes

- `perf` and `ftrace` are powerful but can require elevated permissions.
- `strace` and `ltrace` are great for user-space debugging but introduce some overhead.

## Bonus: Debugging with GDB

The GNU Debugger for inspecting and controlling running programs or analyzing core dumps.

Key Features:

- Step through code line by line, inspect memory and registers.
- Set breakpoints and watchpoints.
- Analyze core dumps after crashes.
- Can attach to running processes or launch programs in debug mode.
- Supports multi-threaded programs and symbols from compiled binaries.

Use Case: In-depth debugging of application logic, crashes, or undefined behavior.

Example usage:

```
gdb ./your_app
(gdb) run
(gdb) break main
(gdb) next
(gdb) print variable
```

Or attach to a running process:

```
gdb -p <pid>
```

Notes

- Requires binaries with debug symbols (-g during compilation).
- Very powerful when used with TUI (gdb -tui) or frontends (e.g., ddd, cgdb).
- Often paired with valgrind, ltrace, strace, or even perf for comprehensive insight.
