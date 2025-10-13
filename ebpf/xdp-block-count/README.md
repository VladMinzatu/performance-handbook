# xdp-block-counte

An XDP-type eBPF program that processes packets at the lowest level (device driver level, before the Linux networking stack sees the packet).

The functionality implemented here is to keep running counts of the most frequent source IPs in an LRU map type and periodically display them in user space.

Additionally, a block list can be statically configured to exclude certain IPs (at compile time for now).

Build instructions:
```
go generate && go build
```

Run:
```
sudo ./xdp-block-count
```