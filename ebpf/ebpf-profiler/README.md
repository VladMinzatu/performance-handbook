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
