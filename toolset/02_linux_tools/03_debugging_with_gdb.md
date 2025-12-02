# Debugging with GDB

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
