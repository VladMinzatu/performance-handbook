# Binary inspection and postmortem analysis

## objdump

`objdump` is a general-purpose binary inspection tool used to display information from object files and executables, including disassembly, sections, and symbols.

Key Features:

- Disassembles machine code to assembly
- Displays sections, symbols, and relocation information
- Works with object files, static libraries, and executables
- Supports multiple architectures and formats

Use Cases:

- Inspect compiler output and generated assembly
- Analyze hot paths identified by profilers
- Debug optimized or partially stripped binaries
- Understand PLT/GOT and linking behavior

Example Usage:

```
objdump -d binary
objdump -S binary # interleave source (if available)
objdump -h binary # list sections
```

Notes:

- For ELF structural details, readelf is often more precise.
- Output can be large; piping to less is common.

## readelf

`readelf` is a specialized utility for inspecting ELF (Executable and Linkable Format) binaries, focusing on headers, sections, and program segments.

Key features:

- Displays ELF headers and metadata
- Inspects sections, segments, and dynamic linking information
- Reads relocation entries and symbol tables
- Does not attempt disassembly

Use cases:

- Diagnose dynamic linking and loader issues
- Inspect ELF layout and ABI details
- Verify binary format, architecture, and dependencies
- Debug startup and runtime loader failures

Example usage:

```
readelf -h binary          # ELF header
readelf -S binary          # section headers
readelf -l binary          # program headers
readelf -d binary          # dynamic section
```

Notes:

- Preferred over objdump for ELF metadata inspection.
- ELF-only (not a general object format tool).

## addr2line

`addr2line` translates instruction addresses into source file names and line numbers using debug symbols.

Key Features:

- Maps addresses to source locations
- Supports inlined function resolution
- Works with executables and shared libraries
- Useful for batch symbolization

Use Cases:

- Symbolize stack traces from crashes or logs
- Resolve addresses from perf or ftrace output
- Post-mortem debugging without an interactive debugger
- Production debugging using symbol files

Example usage:

```
addr2line -e binary 0x4005d4
addr2line -f -C -e binary 0x4005d4
```

Notes:

- Requires debug symbols (-g) for accurate results.
- Often used in scripts and automated tooling.

## nm

`nm` lists symbols from object files, static libraries, and executables, showing their names, addresses, and linkage types.

Key Features:

- Displays defined and undefined symbols
- Identifies symbol visibility and linkage
- Supports demangling of C++ symbols
- Operates on object files and archives

Use Cases:

- Debug linker errors and missing symbols
- Inspect symbol visibility and stripping
- Analyze library exports and ABI surface
- Verify which symbols are included in binaries

Example usage:

```
nm binary
nm -C binary               # demangle C++ symbols
nm -D binary               # dynamic symbols
```

Notes:

- Complements objdump and readelf for symbol analysis.
- Output is frequently filtered or sorted.
