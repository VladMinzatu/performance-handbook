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
Already a substantial gap at the default `WORKERS=32` - atomic is doing ~3.4x the throughput of mutex, before we've even looked at a single syscall trace. However, although it doesn't touch the kernel and it executes as a specialized CPU instruction in kernel space, it is very far from free (as we've seen in a previous lab on cgroups).

Now, in case it's not already running, start the shared analysis container:
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
Zero `futex` calls at `WORKERS=32`.

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
Still zero, at more than 6x the goroutine count. Confirms the first Hypothesis: `atomic.AddInt64` has no kernel path to escalate into regardless of how many goroutines are contending for the same counter - `WORKERS=32` and `WORKERS=200` are indistinguishable from `futex`'s point of view, both zero.

### Extra: why a "zero-syscall" instruction still isn't free under contention

"No kernel involvement" doesn't mean "no contention cost". It just means the contention gets resolved by the *cache-coherency protocol* instead of by the OS. `atomic.AddInt64` still has to read-modify-write one specific memory location, and doing that atomically requires the executing core to hold that cache line in exclusive/modified state for the duration of the operation. When many cores are all atomically incrementing the *same*
counter, every single increment forces the cache line to be invalidated in whatever core last held it and handed over the interconnect (or through shared L3/memory) to the next one - the line "bounces" between cores on every op. That hand-off costs real cycles (far fewer than a syscall + context switch, but far more than an uncontended add), and because only one core can hold the line at a time, it serializes the cores' atomic operations against each other, just via hardware instead of via `sysmon`/`futex`.

To see that effect, sweep `WORKERS` for `MODE=atomic` only and watch `ops/sec`. If there were truly no cost, per-worker throughput would stay flat as `WORKERS` grows:
```sh
docker stop lab-go-counter-atomic
docker rm lab-go-counter-atomic

for W in 1 2 4 8 16 32 64 128; do
  docker run -d --name lab-go-counter-atomic \
    -e MODE=atomic -e WORKERS=$W \
    0104-go-atomics-vs-mutex-contention-counter-atomic:latest >/dev/null
  sleep 4
  OPS=$(docker logs lab-go-counter-atomic | tail -1)
  echo "WORKERS=$W: $OPS"
  docker stop lab-go-counter-atomic >/dev/null
  docker rm lab-go-counter-atomic >/dev/null
done
```
which produces the output:
```sh
WORKERS=1:   ops/sec=256504872 goroutines=2
WORKERS=2:   ops/sec=83358326  goroutines=3
WORKERS=4:   ops/sec=55124834  goroutines=5
WORKERS=8:   ops/sec=29226224  goroutines=9
WORKERS=16:  ops/sec=29234958  goroutines=17
WORKERS=32:  ops/sec=31641002  goroutines=33
WORKERS=64:  ops/sec=31975937  goroutines=65
WORKERS=128: ops/sec=32284810  goroutines=129
```

