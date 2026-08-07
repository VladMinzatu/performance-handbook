## Finding the actual source of the overhead, with pprof

Experiment [02_throttling_effect.md](./02_throttling_effect.md) established that `GOMAXPROCS=2` against a 1-CPU quota loses about 29% of throughput compared to `GOMAXPROCS=1` - and that the loss is bigger than the duty-cycle math alone predicts: both configurations consume essentially the same total CPU-seconds (`usage_usec`), so if every CPU-second bought the same amount of work regardless of thread count, throughput should be about equal. It isn't. Something makes each CPU-second less productive
once there are 2 competing threads instead of 1.

Two hypotheses seemed plausible going in, and both turned out to be measurably too small once actually checked:

- **Kernel-side scheduling overhead** - extra cost from the kernel dequeuing/requeuing two threads instead of one at each throttle
  boundary. Ruled out two ways: `system_usec` in `cpu.stat` was actually *lower* for `GOMAXPROCS=2` than `GOMAXPROCS=1` in our data (the wrong direction for this theory), and a direct measurement from `unthrottle_cfs_rq` (the kernel function that re-enables a throttled cgroup) to the next `sched_switch` for our threads gave a clean 8-16us typical latency - real, but about 2 orders of magnitude too small to explain a multi-millisecond-per-cycle gap.
- **Go runtime overhead around the freeze/resume boundary** - the theory being that `sysmon` or async-preemption machinery reacts  to a goroutine that looks stalled (even though it's just cgroup-frozen), or that resumed threads pay some cache/pipeline warm-up cost. Ruled out by directly timing just the first 5 hash calls after each resume (a low-overhead, targeted trace - not tracing every call): no gradual ramp-up, and even the worst outlier topped out around 4-8us.

Both hypotheses were reasonable-sounding, and both were wrong by a couple of orders of magnitude. Rather than keep guessing at more exotic mechanisms and measuring each one individually, the better move was to ask Go's own profiler where the CPU time is actually going - `pprof` symbolizes natively, sidestepping the "no symbolizer available in this environment" wall that limited the raw `bpftrace`.

A small, temporary addition to `main.go` - capture a CPU profile for the first 8 seconds under the real cgroup limit, then keep running normally:
```go
if f, err := os.Create("/tmp/cpu.pprof"); err == nil {
    pprof.StartCPUProfile(f)
    go func() {
        time.Sleep(8 * time.Second)
        pprof.StopCPUProfile()
        f.Close()
        fmt.Println("pprof profile written to /tmp/cpu.pprof")
    }()
}
```
Run both configurations under the same quota as before, pull the profile out of each container, and analyze with `go tool pprof` - run inside a `golang` container so the whole thing, generation and analysis, stays in Docker rather than depending on anything installed on the host:
```sh
docker build -t cpuburn .

docker run -d --rm --name gmp1-final --cpus=1 -e GOMAXPROCS=1 cpuburn
# wait ~10s for the profile window to close
docker cp gmp1-final:/tmp/cpu.pprof /tmp/pprof-out/gmp1.pprof
docker kill gmp1-final

docker run -d --rm --name gmp2-final --cpus=1 -e GOMAXPROCS=2 cpuburn
docker cp gmp2-final:/tmp/cpu.pprof /tmp/pprof-out/gmp2.pprof
docker kill gmp2-final

docker run --rm -v /tmp/pprof-out:/prof golang:1.23 go tool pprof -top -nodecount=15 /prof/gmp1.pprof
docker run --rm -v /tmp/pprof-out:/prof golang:1.23 go tool pprof -top -nodecount=15 /prof/gmp2.pprof
```

### Results

`GOMAXPROCS=1`:
```
File: cpuburn
Type: cpu
Duration: 8.24s, Total samples = 8.02s (97.34%)
      flat  flat%   sum%        cum   cum%
     5.55s 69.20% 69.20%      5.55s 69.20%  crypto/sha256.sha256block
     0.61s  7.61% 76.81%      6.16s 76.81%  crypto/sha256.block
     0.39s  4.86% 81.67%      6.76s 84.29%  crypto/sha256.(*digest).Write
     0.35s  4.36% 86.03%      2.86s 35.66%  crypto/sha256.(*digest).checkSum
     0.21s  2.62% 88.65%      0.21s  2.62%  crypto/internal/boring/sig.StandardCrypto
     0.20s  2.49% 91.15%      0.20s  2.49%  runtime.duffzero
     0.19s  2.37% 93.52%      7.64s 95.26%  crypto/sha256.Sum256
     0.16s  2.00% 95.51%      0.23s  2.87%  runtime.chanrecv
     0.14s  1.75% 97.26%      0.14s  1.75%  crypto/sha256.(*digest).Reset
     0.12s  1.50% 98.75%      8.02s   100%  main.worker
     0.07s  0.87% 99.63%      0.07s  0.87%  runtime.empty
     0.03s  0.37%   100%      0.26s  3.24%  runtime.selectnbrecv
         0     0%   100%      0.21s  2.62%  crypto/internal/boring.Unreachable (inline)
```

`GOMAXPROCS=2`:
```
File: cpuburn
Type: cpu
Duration: 8.28s, Total samples = 8.07s (97.49%)
      flat  flat%   sum%        cum   cum%
     4.53s 56.13% 56.13%      4.53s 56.13%  crypto/sha256.sha256block
     1.86s 23.05% 79.18%      8.07s   100%  main.worker
     0.44s  5.45% 84.63%      5.53s 68.53%  crypto/sha256.(*digest).Write
     0.42s  5.20% 89.84%      4.95s 61.34%  crypto/sha256.block
     0.23s  2.85% 92.69%      2.94s 36.43%  crypto/sha256.(*digest).checkSum
     0.13s  1.61% 94.30%      0.13s  1.61%  crypto/internal/boring/sig.StandardCrypto
     0.11s  1.36% 95.66%      0.11s  1.36%  runtime.duffzero
     0.09s  1.12% 96.78%      0.12s  1.49%  runtime.chanrecv
     0.08s  0.99% 97.77%      6.02s 74.60%  crypto/sha256.Sum256
     0.05s  0.62% 98.39%      0.05s  0.62%  crypto/sha256.(*digest).Reset
     0.05s  0.62% 99.01%      0.05s  0.62%  runtime.asyncPreempt
     0.05s  0.62% 99.63%      0.17s  2.11%  runtime.selectnbrecv
         0     0% 99.63%      0.13s  1.61%  crypto/internal/boring.Unreachable (inline)
```


The `flat` column is time spent directly in that function, not inside anything it calls. `main.worker`'s own flat time is the tell: **1.50% (0.12s) at `GOMAXPROCS=1` vs. 23.05% (1.86s) at `GOMAXPROCS=2`** - a roughly 15x jump. Correspondingly, `crypto/sha256.Sum256`'s cumulative share of the whole profile drops from 95.26% to 74.60%. Whatever `main.worker` is spending all that extra time doing, it isn't hashing.

Look at what's actually in `worker`:
```go
func worker(counter *uint64, stop <-chan struct{}) {
	data := make([]byte, 64)
	for {
		select {
		case <-stop:
			return
		default:
			sum := sha256.Sum256(data)
			data[0] = sum[0]
			atomic.AddUint64(counter, 1)
		}
	}
}
```
The `select`/channel-check is already accounted for separately (`chanrecv`/`selectnbrecv`/`runtime.empty`), and those stay roughly flat between the two profiles - they're not the story. The other candidate is `atomic.AddUint64(counter, 1)`, which the compiler inlines directly into `worker` rather than giving it its own line in the profile - so its cost shows up as `worker`'s own flat time.

**Both worker goroutines increment the same shared `counter` variable.** With `GOMAXPROCS=1`, only one core ever touches that memory, so each atomic increment is essentially free - the cache line just stays exclusively owned by that core. With `GOMAXPROCS=2`, two different cores are both constantly trying to get exclusive ownership of the *same cache line* to perform the increment. That's a textbook cache-line contention cost (often called "true sharing"): every increment now potentially involves an inter-core cache-coherency transaction instead of a purely local one, and that's dramatically more expensive per operation. This is not a cgroup-throttling effect, a kernel-scheduling effect, or a Go-runtime-bookkeeping effect - it's a completely ordinary multicore performance cost that would show up the moment two threads on two different cores concurrently hammer the same shared atomic variable, with or without any cgroup quota involved at all.
