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

## addr2line

## nm
