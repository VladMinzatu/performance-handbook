## Atomics never touch the kernel

We test Prediction 1: `atomic.AddInt64` should produce zero `futex` syscalls, at the default contention level and at a much higher one - `atomic.AddInt64` has no path into the kernel at all, so there's nothing for contention to escalate into.

First, start both counters:
```sh
docker compose -f compose.yml up -d --build
```

Confirm both started correctly and are producing throughput:
```sh
docker logs lab-go-counter-atomic | head -1
mode=atomic workers=32 GOMAXPROCS=8

docker logs lab-go-counter-mutex | head -1                                          
mode=mutex workers=32 GOMAXPROCS=8

...

docker logs lab-go-counter-atomic | tail -1
docker logs lab-go-counter-mutex | tail -1
ops/sec=44260936 goroutines=33
ops/sec=12927339 goroutines=33

```

In case it's not already running, start the shared analysis container:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
```

Let's trace `futex` syscalls from `counter-atomic` (`WORKERS=32`, the default) over a 10-second window:
```sh
PID=$(docker inspect --format '{{.State.Pid}}' lab-go-counter-atomic)
echo "PID=$PID"
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e 'tracepoint:syscalls:sys_enter_futex /pid == $PID/ { @ = count(); }'"
```
Which produces the output:
```sh
PID=34391

Attaching 1 probe...
@: 0
```

Now let's push `WORKERS` much higher - recreate `counter-atomic` with `WORKERS=200` (a one-off `docker run`, since `compose.yml` pins `WORKERS=32`) and repeat the same trace:
```sh
docker stop lab-go-counter-atomic
docker rm lab-go-counter-atomic

docker run -d --name lab-go-counter-atomic \
  -e MODE=atomic -e WORKERS=200 \
  0104-go-atomics-vs-mutex-contention-counter-atomic:latest

sleep 2

PID=$(docker inspect --format '{{.State.Pid}}' lab-go-counter-atomic)
echo "PID=$PID"
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e 'tracepoint:syscalls:sys_enter_futex /pid == $PID/ { @ = count(); }'"
```
Which produces the output:
```sh
PID=34537
Attaching 1 probe...

@: 0
```
