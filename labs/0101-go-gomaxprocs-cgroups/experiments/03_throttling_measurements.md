## Confirming the throttling model at the scheduling level

The previous experiment derives a specific claim from `cpu.stat`'s aggregate counters alone: with `GOMAXPROCS=2` against a 1-CPU quota, each of the 2 threads runs for roughly the first half of every 100ms period, in parallel, then both sit frozen for the second half - and on top of that duty-cycle loss, there's an additional, smaller tax of about 2.5ms of lost work per thread every time it freezes and resumes (based on the observed loss in throughput). That's arithmetic, not direct observation. In this experiment, we check this against what's actually happening at the OS scheduling level.

The idea is to trace every `sched_switch` event involving the workload's threads, record a timestamp when a thread is switched *out*, and compute the elapsed time when that same thread is next switched back *in*. That's a direct measurement of how long each thread spends off-CPU between runs - if the duty-cycle model above is right, `GOMAXPROCS=2` should show a
cluster of off-CPU gaps around 50ms that `GOMAXPROCS=1` doesn't have.

```sh
docker build -t cpuburn .
docker run -d --rm --name gmp1-sched --cpus=1 -e GOMAXPROCS=1 cpuburn

docker compose -f ../tools/analysis/compose.yml exec -T analysis bash -c "
  bpftrace -e '
    tracepoint:sched:sched_switch
    /args.prev_comm == \"cpuburn\"/
    { @off_start[args.prev_pid] = nsecs; }

    tracepoint:sched:sched_switch
    /args.next_comm == \"cpuburn\" && @off_start[args.next_pid]/
    {
      @off_cpu_ns = hist(nsecs - @off_start[args.next_pid]);
      delete(@off_start[args.next_pid]);
    }
    interval:s:8 { exit(); }
  '
"
docker kill gmp1-sched
```
producing:
```
@off_cpu_ns:
[1K, 2K)               1 |                                                    |
[2K, 4K)               2 |                                                    |
[4K, 8K)               0 |                                                    |
[8K, 16K)              4 |                                                    |
[16K, 32K)             2 |                                                    |
[32K, 64K)             7 |                                                    |
[64K, 128K)            0 |                                                    |
[128K, 256K)           1 |                                                    |
[256K, 512K)          20 |@                                                   |
[512K, 1M)             1 |                                                    |
[1M, 2M)               7 |                                                    |
[2M, 4M)               6 |                                                    |
[4M, 8M)               0 |                                                    |
[8M, 16M)            675 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
```

Then the same thing against `GOMAXPROCS=2`:
```sh
docker run -d --rm --name gmp2-sched --cpus=1 -e GOMAXPROCS=2 cpuburn

docker compose -f ../tools/analysis/compose.yml exec -T analysis bash -c "
  bpftrace -e '
    tracepoint:sched:sched_switch
    /args.prev_comm == \"cpuburn\"/
    { @off_start[args.prev_pid] = nsecs; }

    tracepoint:sched:sched_switch
    /args.next_comm == \"cpuburn\" && @off_start[args.next_pid]/
    {
      @off_cpu_ns = hist(nsecs - @off_start[args.next_pid]);
      delete(@off_start[args.next_pid]);
    }
    interval:s:8 { exit(); }
  '
"
docker kill gmp2-sched
```
producing:
```
@off_cpu_ns:
[512, 1K)             35 |@@                                                  |
[1K, 2K)               4 |                                                    |
[2K, 4K)               7 |                                                    |
[4K, 8K)               3 |                                                    |
[8K, 16K)              3 |                                                    |
[16K, 32K)             4 |                                                    |
[32K, 64K)             3 |                                                    |
[64K, 128K)            0 |                                                    |
[128K, 256K)           0 |                                                    |
[256K, 512K)           0 |                                                    |
[512K, 1M)             1 |                                                    |
[1M, 2M)               0 |                                                    |
[2M, 4M)               0 |                                                    |
[4M, 8M)               1 |                                                    |
[8M, 16M)            645 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[16M, 32M)             2 |                                                    |
[32M, 64M)           160 |@@@@@@@@@@@@                                        |
```

Both configurations share the same dominant cluster around 8-16ms (675 vs. 645 occurrences) - ordinary scheduling preemption that has nothing to do with the cgroup quota, and affects both runs about equally (a thread with nothing else contending for the CPU still gets briefly time-sliced against other work on the machine now and then; this is normal multi-tasking OS behavior, present with or without any cgroup limit at all).

What `GOMAXPROCS=1` doesn't have, and `GOMAXPROCS=2` does, is the second, distinct cluster at 32-64ms - 160 occurrences of it, with nothing comparable anywhere in the single-thread histogram. That's the freeze that the previous experiment reasoned its way to, caught directly this time instead of inferred:
- **The count matches.** With roughly 80 periods elapsing across this 8-second window and both threads freezing together once per period, `2 x ~80 = 160` is almost exactly what got measured.
- **The duration matches.** The bucket itself (32-64ms) brackets the ~51.4ms average freeze duration worked out from `throttled_usec` in `02_throttling_effect.md`.
