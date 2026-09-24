# Go 1.27 portable SIMD: what the vector unit buys, and where it stops

## Background

Go 1.27 adds a portable `simd` package (behind `GOEXPERIMENT=simd`)
alongside the architecture-specific `simd/archsimd`. Types such as
`simd.Float32s` are "at least 128 bits, whatever the hardware offers": on
arm64 that is Neon (128-bit, 4 x float32), on amd64 it can be AVX2/AVX-512,
and elsewhere it falls back to pure-Go emulation. Code is written once
against `Len()` lanes and the compiler picks the width.

Two things make that interesting to measure rather than just adopt:

- **Vectorising is not the same as being fast.** A dot product is a
  reduction: each add depends on the previous one, so throughput is set by
  instruction *latency*, not by how many lanes fit in a register. A
  kernel like elementwise add has no such chain and is limited by
  load/store bandwidth instead. The two respond to SIMD very differently.
- **Go's compiler does not auto-vectorise.** Floating-point addition is not
  associative, so even the scalar loop can't be reordered for you. Any
  speedup has to be asked for explicitly, either with lanes (SIMD) or by
  hand-splitting accumulators.

The lab compares four dot-product kernels and two elementwise-add kernels
in `kernels.go` / `kernels_simd.go`, then the same operations in NumPy.

| Kernel | What it is |
|---|---|
| `DotScalar` | one accumulator, serial dependency chain |
| `DotScalar4` | four scalar accumulators (chain broken by hand) |
| `DotSIMD1` | one vector accumulator (`MulAdd`) |
| `DotSIMD4` | four vector accumulators |
| `AddScalar` / `AddSIMD` | `dst = x + y`, no dependency chain |

Working-set sizes are `n = 1Ki .. 16Mi` float32s, chosen to land in L1, L2
and DRAM.

## Hypotheses

**1 - Dot product is latency-bound, so multiple accumulators matter as
much as lanes.** `DotScalar4` should beat `DotScalar` by roughly the FMA
latency divided by its issue interval (several x) with no SIMD at all.
`DotSIMD1` should gain about the lane count over `DotScalar` but stay
latency-bound; `DotSIMD4` should then gain again over `DotSIMD1`.

**2 - Elementwise add is bandwidth-bound, so SIMD helps far less than the
lane count suggests.** Once data is beyond L1/L2 the scalar and SIMD adds
should converge on memory bandwidth; inside L1 SIMD should win clearly.

**3 - The portable API compiles to real Neon, with a cost.** Disassembly
of `DotSIMD4` should show `FMLA` on `V` registers in the hot loop. The
package supports several vector widths in one binary, so we expect some
dispatch overhead (a width check and a call) around, not inside, the loop.

**4 - Bounds checks are a real tax on the scalar side.** `DotScalar4`
indexes `x[i+1]`, `y[i+3]` and so on; the compiler can't always prove them
in range. We expect extra compare-and-branch instructions per iteration
that the SIMD loads avoid, so part of any SIMD "win" is check elimination.

**5 - NumPy is fast for a different reason.** `np.dot` should be far ahead
of `(x*y).sum()` (fused, BLAS-backed, no temporary), and `x + y` should
be close to bandwidth-bound. Go's SIMD kernels should land in the same
order of magnitude as NumPy's elementwise ops, but NumPy's `x + y` also
pays an allocation for the output.

## Setup

```sh
docker compose up -d --build
```

The image contains the compiled Go test binary (`simd.test`, built with
`GOEXPERIMENT=simd`), NumPy, `perf` and binutils, on an arm64 Linux
container limited to 2 CPUs.

Correctness first - all kernels must agree with the scalar reference:
```sh
GOEXPERIMENT=simd go1.27.1 test ./...
```

Go benchmarks:
```sh
docker exec lab-go-simd simd.test -test.run xxx -test.bench . -test.count 5
```

NumPy, same sizes and the same MB/s accounting:
```sh
docker exec lab-go-simd python3 /bench.py
```

Disassembly of a kernel, in native ARM syntax, from the container's binary:
```sh
docker exec lab-go-simd sh -c 'objdump -d --no-show-raw-insn --disassemble="simdlab.DotSIMD4@simd128" /usr/local/bin/simd.test'
```
and in Go's own assembler syntax on the host:
```sh
GOEXPERIMENT=simd go1.27.1 test -c -o /tmp/simd.test
go1.27.1 tool objdump -s 'simdlab.DotSIMD4' /tmp/simd.test
```

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose down
```
