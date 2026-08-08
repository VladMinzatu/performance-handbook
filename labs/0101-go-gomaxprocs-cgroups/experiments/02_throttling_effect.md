## Throttling effect

Let's create the image to run some tests:
```sh
docker build -t cpuburn .
```

```sh
docker run -d --rm --name gmp1-test --cpus=1 -e GOMAXPROCS=1 cpuburn

docker logs gmp1-test                       
GOMAXPROCS=1 NumCPU=8 numWorkers=1
ops/sec=15169901 goroutines=2
ops/sec=15381939 goroutines=2
ops/sec=14298684 goroutines=2

docker kill gmp1-test
```

```sh
docker run -d --rm --name gmp2-test --cpus=1 -e GOMAXPROCS=2 cpuburn

docker logs gmp2-test  
GOMAXPROCS=2 NumCPU=8 numWorkers=2
ops/sec=11384401 goroutines=3
ops/sec=10369581 goroutines=3
ops/sec=10338129 goroutines=3
ops/sec=10013226 goroutines=3
ops/sec=11045344 goroutines=3
ops/sec=10752149 goroutines=3

docker kill gmp2-test
```

### Findings

Averaging each run: ~14.95M ops/sec at `GOMAXPROCS=1` versus ~10.65M at `GOMAXPROCS=2` - a 29% loss, against the *same* 1-CPU quota, despite the second run nominally having twice as many workers trying to do the work.

To see why, repeat each run with a check of the cgroup's own accounting right after:
```sh
docker exec <container> cat /sys/fs/cgroup/cpu.stat
```
```sh
GOMAXPROCS=1:
usage_usec 6091916
nr_periods 61
nr_throttled 35
throttled_usec 14776

GOMAXPROCS=2:
usage_usec 6228485
nr_periods 62
nr_throttled 61
throttled_usec 6266616
```

**First: the quota ceiling gets hit far more often with 2 threads.** Only about half the periods reach it with a single thread (35/61 - occasional, minor overshoots), but essentially *every* period does with two (61/62). That's the direct mechanical consequence of the quota being a fixed CPU-time budget per period (100ms of CPU-time per 100ms period, for a 1-CPU limit): with 2 always-busy threads running in parallel, that 100ms budget gets consumed in about half the wall-clock time it takes a single thread to use it up, leaving the rest of every period for the kernel to freeze the whole cgroup - not slow it down, stop it - until the next period's budget arrives.

The `throttled_usec` numbers let us work out *how* that time actually splits, rather than just asserting it. `throttled_usec` for `GOMAXPROCS=2` is 6,266,616us - which is *larger* than the roughly 6.2s of real wall-clock time the whole run took (62 periods x 100ms). That's only possible if, like `usage_usec`, this counter sums across every thread being throttled simultaneously rather than measuring wall-clock time once - i.e. two threads frozen together for 50ms each contribute 100ms to the total, not 50ms. Dividing back out: 6,266,616us over 61 throttled periods is ~102,732us of *summed* freeze time per period: split across 2 threads,that's ~51.4ms frozen, per thread, per period - almost exactly half. In other words, each of the 2 threads gets to run for roughly the first half of every period, in parallel, burns through the shared budget together in that time, and then both sit frozen for the second half. That's a clean, mechanical explanation for *some* throughput loss: each individual thread
is only actually executing about half the time.

**But that alone doesn't explain the size of the gap.** Both configurations consume essentially the *same* total CPU-seconds
(`usage_usec`: 6,091,916 vs. 6,228,485 - `GOMAXPROCS=2` used marginally *more*, if anything). If splitting that same budget across 2 threads instead of 1 came at no extra cost, both runs should produce essentially the same amount of work - the arithmetic predicts `GOMAXPROCS=2` should do about `14.95M x (6,228,485/6,091,916)` ~= 15.29M ops/sec, matching (or slightly beating) `GOMAXPROCS=1`. It actually does 10.65M - about 4.64M ops/sec short of that prediction, or roughly 74,800 "missing" ops per period.

Converting that gap into time (using `GOMAXPROCS=1`'s own rate as the baseline cost of one hash: ~1 CPU-second / 14.95M ops =~ 67ns/op), ~74,800 missing ops per period is about 5ms of otherwise-achievable work disappearing from every single period - split across the 2 threads that each freeze and resume once per period, that's roughly **2.5ms of lost work per thread, per throttle-freeze-and-resume cycle**. That's a small but real, and remarkably consistent, tax that a continuously-running thread (`GOMAXPROCS=1`, which almost never gets frozen) never pays. The question is where does it come from: could be because the on-CPU windows are slightly shorter than modeled, most likely from scheduling wake-up latency. But that is unlikely (should be orders of magnitude faster). Or maybe from reduced per-call throughput once running, but that is also unlikely.

Nothing here depends on what specific hardware this happens to run on - whatever gets evicted or has to be rebuilt each time a stopped thread resumes (cache contents, pipeline state, the Go runtime's own bookkeeping for a rescheduled goroutine) costs *something*, and a thread that freezes and resumes roughly once every 100ms pays that cost roughly once every 100ms, while a thread that's never interrupted doesn't pay it at all.

This is all derived from `cpu.stat`'s aggregate counters, by arithmetic -
see [03_throttling_measurements.md](./03_throttling_measurements.md) for
direct, OS-level scheduling measurements that confirm it.

### Background: Why this is specifically a CPU-time effect

Note that this only shows up for CPU-bound work, since it's tempting to assume any kind of "too many goroutines" situation behaves similarly, but that's not the case. The cgroup CPU controller is purely a CPU-*time* accounting mechanism: it tracks how many CPU-seconds a cgroup's threads actually spend running on a CPU, and throttles once that crosses the configured quota for the current period. 

That means a goroutine blocked in `time.Sleep()`, waiting on a channel, or blocked on network I/O isn't consuming any CPU time while it waits, so it contributes nothing toward exhausting the quota - it simply isn't part of what this mechanism measures. A service with hundreds of mostly-idle goroutines handling slow requests could have its `GOMAXPROCS` set just as "wrong" as this lab's workload, and never once get throttled by the CPU controller, because it's never actually trying to consume more CPU-seconds
than its quota allows - the goroutines are waiting, not running. The throttling and throughput loss demonstrated here is specific to workloads that are genuinely, continuously CPU-bound; for anything I/O- or timer-bound, a wrong `GOMAXPROCS` might still matter for other reasons (e.g. scheduling fairness or memory overhead from excess OS threads), but it would not show up as CFS bandwidth throttling, because there'd be no CPU-time to throttle in the first place.
