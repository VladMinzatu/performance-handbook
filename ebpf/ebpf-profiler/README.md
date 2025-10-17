# ebpf-profiler

A basic continuous profiler written in [ebpf-go](https://github.com/cilium/ebpf).

The profiler requires a mandatory `pid` flag, denoting the process to be profiled.

Build with:
```
go generate && go build
```

Run:
```
sudo ./ebpf-profiler -pid=1234
```

This implementation uses `BPF_MAP_TYPE_RINGBUF` (alternatively, could have used `PERF_EVENT_ARRAY`) and sends raw events to userspace (similar to how `perf` works).

To better take advantage of eBPF's performance potential (and make this project suitable for real production continuous profiling), in-kernel aggregation with a map type like `BPF_MAP_TYPE_PERCPU_HASH` or `BPF_MAP_TYPE_LRU_HASH` should be used, with user space reading the contents only as often as needed.
