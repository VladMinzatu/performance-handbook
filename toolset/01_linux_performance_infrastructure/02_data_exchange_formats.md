# Data and visualisation interoperability

We talk about this here as another topic where various tools meet, although this is not as much a Linux-specific topic.

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
