# compactsnoop

[compactsnoop](https://github.com/iovisor/bcc/blob/master/tools/bpflist.py) traces page compaction events (which is when the kernel tries to free contiguous physical memory for large allocations).

The tool can be run like so:
```
sudo ./compactsnoop
```

Turns out, it is pretty hard to trigger compaction events. I included a small script to do allocations aggressively (many large allocations, ensuring physical allocation by touching every page and triggering gc on each iteration). It can be run with `go run test.go`. And I did catch this following output:
```
COMM           PID    NODE ZONE         ORDER MODE      LAT(ms)           STATUS
kcompactd0     47     0    ZONE_NORMAL  9     SYNC        0.652  partial_skipped
```

The typical use case for this (compaction) is when there is high memory pressure (frequent allocations and frees of various sizes in long running workloads, like DBs and services), large allocations and/or low memory.

