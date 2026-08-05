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

That's because, although we have 8 cores available on the test machine, and `lab-go-cpuburn-default` thinks it can grab all of them, the cgroup throttling is still enforced and we can see this from the fact that `lab-go-cpuburn-default` has a much higher rate of `nr_throttled` to `nr_periods` (basically 1 to 1) compared to `lab-go-cpuburn-fixed`:

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