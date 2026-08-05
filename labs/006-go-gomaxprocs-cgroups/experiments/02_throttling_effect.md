## Throttling effect

Let's check the throughput of the two containers after they've been working for a while:
```sh
docker logs lab-go-cpuburn-default | tail -5
ops/sec=7203054 goroutines=9
ops/sec=7145298 goroutines=9
ops/sec=7108741 goroutines=9
ops/sec=7764077 goroutines=9
ops/sec=7047468 goroutines=9
```
and
```sh
docker logs lab-go-cpuburn-fixed | tail -5
ops/sec=20784468 goroutines=3
ops/sec=19952827 goroutines=3
ops/sec=20320202 goroutines=3
ops/sec=21136091 goroutines=3
ops/sec=19571130 goroutines=3
```

Note that they are both doing busy pure-CPU bound work at the with GOMAXPROCs number of workers each.

And what we notice is that `lab-go-cpuburn-fixed` has much better throughput: its 2 workers/goroutines update the counter roughly 20M times every second, while `lab-go-cpuburn-default` only manages around 7M updates with its 8 workers/goroutines.

This is actually a much bigger difference than we could have expected, as it should be mainly scheduling overhead or managing more workers for the default version. We can dig deeper into what is happening:
```sh
> docker exec lab-go-cpuburn-default cat /sys/fs/cgroup/cpu.stat
usage_usec 803061268
user_usec 801256611
system_usec 1804656
nice_usec 0
nr_periods 4016
nr_throttled 4015
throttled_usec 1601744831
nr_bursts 0
burst_usec 0

> docker exec lab-go-cpuburn-fixed cat /sys/fs/cgroup/cpu.stat
usage_usec 800072898
user_usec 799192783
system_usec 880114
nice_usec 0
nr_periods 4015
nr_throttled 2157
throttled_usec 2932223
nr_bursts 0
burst_usec 0
```

To make sense of these numbers, it helps to know how the kernel actually enforces a `cpus: "2"` limit. It isn't "pin this group to 2 specific cores" (that's a different mechanism, `cpuset`) - by default the scheduler is still free to run the group's threads on any of the host's cores. Instead, the limit is a *time budget*: for a 2-CPU limit, the cgroup is allowed at most 200ms of total CPU-time - summed across however many cores it actually runs on - in every 100ms period. That's just as satisfied by 2 cores running flat out for the whole period as by, say, 8 cores each running for 25ms.

That's exactly why `cpuburn-default`'s 8 always-busy threads burn through the budget so fast: spread across up to 8 cores at once, they can consume 200ms of aggregate CPU-time in roughly the first 25ms of wall-clock time within each 100ms period. Once the budget hits zero, the kernel doesn't slow the threads down - it stops scheduling every one of them entirely for the rest of that period, no matter how many cores are sitting idle. That hard stop is what `nr_throttled` counts, not "ran slower." Since this exhaust-then-block cycle repeats in essentially every single period for `cpuburn-default`, its `nr_throttled`/`nr_periods` ratio lands near 1:1. `cpuburn-fixed`'s 2 threads, by contrast, consume the budget at close to the rate it refills, so most periods finish without ever hitting zero - its much lower ratio reflects only occasional, brief overshoots from ordinary scheduling jitter,not a workload that's structurally oversubscribed.

But that doesn't explain the difference in throughput! We can look at the respective `system_usec`s and see that indeed `lab-go-cpuburn-default` does spend more system time, presumably attributable (in part) to the overhead of scheduling more workers. But that is less than 1% of the `usage_usec` either way - definitely not explaining the 3x difference in throughput.

### A middle data point: GOMAXPROCS=4

Before going further, it's worth checking whether the loss scales the way a "wrong number of cores" story would predict. This machine's host is an Apple M2: 4 Performance cores + 4 Efficiency cores. With `GOMAXPROCS=8`, at least 4 of the 8 threads *must* run on the slower E-cores - there's nowhere else to put them. With exactly `GOMAXPROCS=4`, all 4 threads could in principle fit entirely on the 4 P-cores. If core type were the whole story, throughput at `GOMAXPROCS=4` should land much closer to the 2-thread case than to the 8-thread one.

```sh
docker run -d --rm --name gmp4-test --cpus=2 -e GOMAXPROCS=4 <the cpuburn image>

docker logs <container_id>
...
GOMAXPROCS=4 NumCPU=8 numWorkers=4
ops/sec=9223328 goroutines=5
ops/sec=8450994 goroutines=5
ops/sec=7939271 goroutines=5
ops/sec=8263649 goroutines=5
ops/sec=8186327 goroutines=5
```
It doesn't. ~9M ops/sec is barely better than `GOMAXPROCS=8`'s ~7M, and less than half of `GOMAXPROCS=2`'s ~20M. Its `cpu.stat` also shows the same "throttled in nearly every period" signature (`nr_throttled`/`nr_periods` ≈ 81/83) as the 8-thread case, just with a shorter freeze per period (4 threads exhaust the 200ms budget around the 50ms mark instead of the 25ms mark). So whatever is driving the loss doesn't cleanly track "how many threads exceed the P-core count" - it tracks something closer to "is this oversubscribed at all," and going from 4x to 2x oversubscribed only recovers a small fraction of the gap.

### Isolating it: does an individual hash call itself get slower?

Everything so far is throughput and aggregate `cpu.stat` counters - consistent with *some* per-operation slowdown, but not direct evidence of one. To check this directly, we can time individual `crypto/sha256.Sum256` calls with a uprobe/uretprobe pair, keyed by thread id so concurrent calls on different OS threads don't get mixed up:
```sh
PID=$(docker inspect -f '{{.State.Pid}}' <container>)
docker compose -f ../tools/analysis/compose.yml exec -T analysis bash -c "
  bpftrace -e '
    uprobe:/proc/${PID}/root/usr/local/bin/cpuburn:crypto/sha256.Sum256
    { @start[tid] = nsecs; }
    uretprobe:/proc/${PID}/root/usr/local/bin/cpuburn:crypto/sha256.Sum256
    /@start[tid]/
    {
      @latency_ns = hist(nsecs - @start[tid]);
      delete(@start[tid]);
    }
    interval:s:5 { exit(); }
  '
"
```
Running one container at a time (stopping the others first) - all three services build from the same image, and a uprobe attaches by file/inode, not by process, so it can fire across containers that happen to share the same underlying binary layer on disk.

`GOMAXPROCS=2` (fixed):
```
@latency_ns:
[512, 1K)        8619696 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[1K, 2K)          114309 |                                                    |
[2K, 4K)            2033 |
...
[4M, 8M)               1 |
```

`GOMAXPROCS=4`:
```
@latency_ns:
[512, 1K)         334519 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[1K, 2K)           45589 |@@@@@@@
...
[1M, 2M)               1 |
```

`GOMAXPROCS=8` (default):
```
@latency_ns:
[512, 1K)         118173 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@     |
[1K, 2K)          128224 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[2K, 4K)           21281 |@@@@@@@@
...
[32M, 64M)              9 |
[64M, 128M)             1 |
```

Two distinct things show up here, and it's worth not conflating them:

- **The extreme tail** (milliseconds-long calls, only present for `GOMAXPROCS=8`) is the duty-cycle effect caught in the act: a hash call that started, got frozen mid-execution when the cgroup's quota ran out, and only finished once the next period began. That's the same throttling `cpu.stat` already showed us, just visible now at the level of one specific in-flight operation instead of an aggregate counter.
- **The bulk of the distribution** - excluding that tail entirely - still shifts. The fraction of calls landing in the slower 1-2us bucket (versus the faster 512ns-1us one) climbs steadily with oversubscription:

  | | % of calls in the slower (1-2us) bucket |
  |---|---|
  | GOMAXPROCS=2 | 1.3% |
  | GOMAXPROCS=4 | 12.0% |
  | GOMAXPROCS=8 | 52.0% |

This is calls that completed without ever hitting a freeze boundary, still taking measurably longer, more often, as more threads compete - a real per-operation efficiency loss, cleanly separable from the freeze/duty-cycle story, and scaling smoothly with the degree of oversubscription rather than behaving like a fixed, binary cost.

### Conclusion

The throughput gap is the sum of two distinct, additive effects, not one:
1. **Duty-cycle loss** - oversubscribed threads spend a large fraction of every period frozen entirely (the `nr_throttled`/`cpu.stat` story), visible directly as multi-millisecond outliers in the per-call latency data.
2. **Per-operation efficiency loss** - even the calls that *do* run, without being frozen mid-flight, get measurably slower as more threads compete for the same quota, in a dose-dependent way (1.3% -> 12.0% -> 52.0% of calls landing in the slower bucket as oversubscription goes from 1x to 2x to 4x).

What we can't pin down further with the tools available here is the exact physical mechanism behind effect (2). This host's chip has 4 Performance + 4 Efficiency cores, and being forced onto a slower E-core is a plausible contributor - but the middle `GOMAXPROCS=4` data point (which should avoid E-cores entirely, if that were the whole story) argues it's not the complete picture, and possibly not even the dominant one. Cache/TLB pressure from more concurrently-active threads, or per-core frequency scaling under heavier simultaneous load, are equally plausible and not distinguishable from here.
