## Build image

```
docker build -t bpftrace-hol .
```

## Run

```
docker run --rm -it \
    --privileged \
    --pid=host \
    -v /sys:/sys \
    -v /lib/modules:/lib/modules:ro \
    -v /usr/src:/usr/src:ro \
    bpftrace-hol
```

```
bpftrace --version
```

```
bpftrace -l 'tracepoint:syscalls:*open*'
```