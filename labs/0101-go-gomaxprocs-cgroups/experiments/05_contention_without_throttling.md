## Isolating the contention cost from throttling entirely

Everything in [04_pprof_confirmation.md](./04_pprof_confirmation.md) was measured *while* the cgroup was actively throttling - `GOMAXPROCS=2` against a 1-CPU quota, an oversubscribed, heavily-freezing scenario. That raises a fair question: is the atomic-contention finding actually a general multicore effect, or could it somehow be an artifact of measuring it inside an already-throttled environment?

The cleanest way to check is to remove throttling from the equation entirely: match the quota to the thread count exactly, in both
configurations, so there's nothing to throttle. If pure quota/duty-cycle mechanics were the only cost, two threads with two full CPUs of quota should scale to almost exactly 2x the throughput of one thread with one CPU. Any shortfall from that can only be the contention cost.

```sh
docker run -d --rm --name gmp1-nothrottle --cpus=1 -e GOMAXPROCS=1 cpuburn

docker logs gmp1-nothrottle
ops/sec=14929103 goroutines=2
ops/sec=14912664 goroutines=2
ops/sec=14846075 goroutines=2
ops/sec=14687563 goroutines=2
ops/sec=14905746 goroutines=2

docker exec gmp1-nothrottle cat /sys/fs/cgroup/cpu.stat
usage_usec 8128606
nr_periods 81
nr_throttled 7
throttled_usec 8704

docker kill gmp1-nothrottle
```
```sh
docker run -d --rm --name gmp2-nothrottle --cpus=2 -e GOMAXPROCS=2 cpuburn

docker logs gmp2-nothrottle
ops/sec=21952605 goroutines=3
ops/sec=23020401 goroutines=3
ops/sec=22951946 goroutines=3
ops/sec=23011644 goroutines=3
ops/sec=23032125 goroutines=3

docker exec gmp2-nothrottle cat /sys/fs/cgroup/cpu.stat
usage_usec 16212354
nr_periods 81
nr_throttled 31
throttled_usec 15616

docker kill gmp2-nothrottle
```

### Confirming there's nothing to throttle here

`throttled_usec` is 8,704us for the 1-CPU/1-thread run and 15,616us for the 2-CPU/2-thread run - both under 16ms out of an ~8s window, i.e. well under 0.2% of the total time either way. `nr_throttled` being nonzero (7/81 and 31/81 periods) just reflects the same kind of brief, incidental overshoots seen throughout this lab whenever a thread count roughly matches its quota - nothing like the "throttled in nearly every period" signature of genuine oversubscription. Both configurations are, for practical purposes, unthrottled.

### The result

Averaging each run: ~14.86M ops/sec at `GOMAXPROCS=1`/`cpus=1`, ~22.79M at `GOMAXPROCS=2`/`cpus=2`.
```
Perfect 2x scaling would predict: 14.86M x 2 = 29.71M ops/sec
Actual:                                        22.79M ops/sec
Scaling factor achieved:                       1.534x (not 2x)
Shortfall from perfect scaling:                23.3%
```

With no meaningful throttling in either configuration, doubling the threads and doubling the CPU quota together should come close to doubling throughput, if every CPU-second were equally productive regardless of thread count. It doesn't - two threads on two cores achieve only 1.53x the throughput of one thread on one core, a 23.3% shortfall from perfect
scaling.

That number lines up almost exactly with the ~23% share of CPU time `pprof` attributed to `main.worker`'s own flat time. The strong suspect here is the inlined `atomic.AddUint64` call, but it may not be the only contributor. Nevertheless, this experiment does prove the lack of an effect from the throttling: two different experiments - one riddled with cgroup throttling, one with essentially none at all - converge on the same number.