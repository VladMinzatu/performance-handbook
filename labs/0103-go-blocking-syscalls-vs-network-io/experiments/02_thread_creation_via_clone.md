## Watching the threads actually get created

We test here Prediction 2: the thread growth with `MODE=syscall` showed in the previous experiment isn't instant - Go doesn't spin up a new OS thread the moment a goroutine enters a slow syscall. `sysmon` has to first notice, on its own periodic sweep, that a `P` has been sitting in syscall state too long, retake it, and only then does a new M get created to make that `P` productive again. So tracing `clone`/`clone3` on `worker-syscall` during its startup ramp (from 0 blocked goroutines up to `WORKERS=2000`) should show real thread-creation syscalls tracking that growth - in sharp contrast to `worker-netpoll` (or `worker-timer`), which should show close to zero `clone` calls during the exact same kind of ramp.

This needs the shared `analysis` container, so we start it if it isn't already running:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
```

Now recreate the `lab-go-worker-syscall` container from scratch so we can trace its ramp from zero, starting the trace a couple of seconds before the container exists so the probe is already attached when it starts:
```sh
docker stop lab-go-worker-syscall
docker rm lab-go-worker-syscall

(sleep 2; docker compose -f compose.yml up -d worker-syscall) &
docker exec lab-analysis sh -c "timeout -s INT 15 bpftrace -e 'tracepoint:syscalls:sys_enter_clone /comm == \"worker\"/ { @[\"clone\"] = count(); } tracepoint:syscalls:sys_enter_clone3 /comm == \"worker\"/ { @[\"clone3\"] = count(); }'"
wait
```
and this produces the output:
```sh
[1] 45452
Attaching 2 probes...
[+] up 1/1
 ✔ Container lab-go-worker-syscall Started                                                                                                                                                                                                   0.2s
[1]  + done       ( sleep 2; docker compose -f compose.yml up -d worker-syscall; )

@[clone]: 2009
```

We can also confirm it actually reached the same steady state as the previous experiment:
```sh
docker exec lab-go-worker-syscall sh -c 'grep Threads /proc/1/status'
Threads:	2010
```

Next, let's do the same thing for `worker-netpoll` to contrast:
```sh
docker stop lab-go-worker-netpoll
docker rm lab-go-worker-netpoll

(sleep 2; docker compose -f compose.yml up -d worker-netpoll) &
docker exec lab-analysis sh -c "timeout -s INT 15 bpftrace -e 'tracepoint:syscalls:sys_enter_clone /comm == \"worker\"/ { @[\"clone\"] = count(); } tracepoint:syscalls:sys_enter_clone3 /comm == \"worker\"/ { @[\"clone3\"] = count(); }'"
wait
```

which produces the output:
```sh
[1] 45506
Attaching 2 probes...
[+] up 2/2
 ✔ Container lab-go-delay-server   Running                                                                                                                                                                                                   0.0s
 ✔ Container lab-go-worker-netpoll Started                                                                                                                                                                                                   0.2s
[1]  + done       ( sleep 2; docker compose -f compose.yml up -d worker-netpoll; )

@[clone]: 15
```
And again to contrast the end state as well:
```sh
docker exec lab-go-worker-netpoll sh -c 'grep Threads /proc/1/status'
Threads:	16
```

But this just pretty much reconfirms the previous hypothesis with different measurements. What about the growth curve of the number of threads for the syscall worker?

What would distinguish "instant" from "gradual, sysmon-tick-paced" thread creation is the *timing distribution* of the individual `clone()` calls themselves:

```sh
docker stop lab-go-worker-syscall && docker rm lab-go-worker-syscall

docker exec lab-analysis sh -c "bpftrace -e '
BEGIN { @start = nsecs; }
tracepoint:syscalls:sys_enter_clone /comm == \"worker\"/ {
  \$ms = (nsecs - @start) / 1000000;
  @h = lhist(\$ms, 0, 6000, 100);
}
interval:s:12 { exit(); }
'" &
sleep 3
docker compose -f compose.yml up -d worker-syscall
wait
```
And that produces a timeline of our 2000+ threads being created gradually:
```sh
@h: 
[3200, 3300)         180 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[3300, 3400)         142 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@           |
[3400, 3500)         122 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                 |
[3500, 3600)         161 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@      |
[3600, 3700)         122 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                 |
[3700, 3800)         102 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                       |
[3800, 3900)          94 |@@@@@@@@@@@@@@@@@@@@@@@@@@@                         |
[3900, 4000)         122 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                 |
[4000, 4100)          58 |@@@@@@@@@@@@@@@@                                    |
[4100, 4200)          64 |@@@@@@@@@@@@@@@@@@                                  |
[4200, 4300)         108 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                     |
[4300, 4400)         135 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@             |
[4400, 4500)          98 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@                        |
[4500, 4600)          44 |@@@@@@@@@@@@                                        |
[4600, 4700)          96 |@@@@@@@@@@@@@@@@@@@@@@@@@@@                         |
[4700, 4800)          73 |@@@@@@@@@@@@@@@@@@@@@                               |
[4800, 4900)          75 |@@@@@@@@@@@@@@@@@@@@@                               |
[4900, 5000)          47 |@@@@@@@@@@@@@                                       |
[5000, 5100)          71 |@@@@@@@@@@@@@@@@@@@@                                |
[5100, 5200)          45 |@@@@@@@@@@@@@                                       |
[5200, 5300)          24 |@@@@@@                                              |
[5300, 5400)          25 |@@@@@@@                                             |
[5400, 5500)           0 |                                                    |
[5500, 5600)           0 |                                                    |
[5600, 5700)           0 |                                                    |
[5700, 5800)           1 |                                                    |
```

The bucket counts sum to exactly 2009 - the same total the simple `count()` trace found earlier, so the histogram is confirmed to be capturing the complete set of events. And it directly confirms Prediction 2: thread creation is spread across roughly 2.5 seconds (buckets 3200 through 5700, i.e. ~200ms to ~2800ms after the container actually starts, once the `BEGIN`-to-container-start offset from `sleep 3` is subtracted out) - not one instantaneous jump, which is exactly what "goroutine enters a syscall" being merely the *trigger* for eventual `sysmon` retake, rather than the cause of immediate thread creation, predicts.

However, given the timing observed here and the fact that we have WORK_MS=200 for our worker, there are 2 competing contributors to spreading out the thread creation:

1. **A hard throughput ceiling.** There are only `GOMAXPROCS=8` `P`s, and `sysmon`'s retake sweep can only ever hand off a stale `P` at its own periodic cadence. Onboarding 2000 never-yet-scheduled goroutines onto their first dedicated M, one retake event at a time, takes real wall-clock time no matter what - this alone would spread thread creation out.
2. **Reuse competing with onboarding.** `WORK_MS=200ms` is short relative to the ~2.6s total ramp - so some *early*-onboarded goroutines are already finishing their first sleep and freeing their M for reuse *while later goroutines are still waiting for their first M ever*. Every retake event that reactivates one of these already-warm Ms is a retake event *not* spent onboarding a new goroutine, which would slow the ramp down and could plausibly produce exactly the declining shape we're seeing, independent of (1).

A single histogram at `WORK_MS=200` can't tell these apart - both predict "gradual, not instant," and either could produce a declining rate over time.

To disentangle this, let's rerun with WORK_MS set to 1 minute:
```sh
docker stop lab-go-worker-syscall
docker rm lab-go-worker-syscall

docker exec lab-analysis sh -c "bpftrace -e '
BEGIN { @start = nsecs; }
tracepoint:syscalls:sys_enter_clone /comm == \"worker\"/ {
  \$ms = (nsecs - @start) / 1000000;
  @h = lhist(\$ms, 0, 6000, 100);
}
interval:s:12 { exit(); }
'" &
sleep 3
docker run -d --name lab-go-worker-syscall --network labnet \
  -e MODE=syscall -e WORKERS=2000 -e WORK_MS=60000 \
  0103-go-blocking-syscalls-vs-network-io-worker-syscall:latest
wait
```

And now the distribution looks a little different, but still with a spread:
```sh
@h: 
[2900, 3000)         148 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@            |
[3000, 3100)         143 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@             |
[3100, 3200)         184 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@  |
[3200, 3300)         190 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[3300, 3400)         169 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@      |
[3400, 3500)         139 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@              |
[3500, 3600)         153 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@           |
[3600, 3700)         168 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@       |
[3700, 3800)         152 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@           |
[3800, 3900)         146 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@             |
[3900, 4000)         177 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@    |
[4000, 4100)         137 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@               |
[4100, 4200)          96 |@@@@@@@@@@@@@@@@@@@@@@@@@@                          |
[4200, 4300)           0 |                                                    |
[4300, 4400)           0 |                                                    |
[4400, 4500)           0 |                                                    |
[4500, 4600)           0 |                                                    |
[4600, 4700)           0 |                                                    |
[4700, 4800)           0 |                                                    |
[4800, 4900)           0 |                                                    |
[4900, 5000)           1 |                                                    |
[5000, 5100)           0 |                                                    |
[5100, 5200)           0 |                                                    |
[5200, 5300)           0 |                                                    |
[5300, 5400)           0 |                                                    |
[5400, 5500)           0 |                                                    |
[5500, 5600)           0 |                                                    |
[5600, 5700)           0 |                                                    |
[5700, 5800)           0 |                                                    |
[5800, 5900)           0 |                                                    |
[5900, 6000)           0 |                                                    |
[6000, ...)            1 |                                                    |
```

This shows that mechanism (2) - reuse competing with onboarding - was a real contributor, not just noise. With `WORK_MS=60000`, no goroutine's sleep can possibly complete within this trace's few-second window, so reuse is structurally impossible here: every single retake event *has* to be onboarding a genuinely new goroutine. Two things change compared to the `WORK_MS=200` run:

- **The ramp finishes faster, not slower.** 2002 of the 2004 total events land in one tight, contiguous cluster spanning just 1.3 seconds (2900ms to 4200ms) - versus ~2.6 seconds for `WORK_MS=200`. That's the opposite of what "longer blocks should make thread creation harder" would naively suggest, but it makes sense under mechanism (2): in the `WORK_MS=200` run, some fraction of each retake tick's limited throughput was being spent reactivating already-onboarded goroutines cycling back into another sleep, competing with (and slowing down) the onboarding of goroutines that had never run yet. Remove that competition entirely and 100% of the retake throughput goes toward onboarding new goroutines, finishing sooner.
- **The shape is roughly flat, not declining.** Bucket counts sit in a fairly tight band (96-190) across the whole cluster, rather than the clear decay from ~180 down to ~24 seen with `WORK_MS=200` - then it just stops abruptly once the population is exhausted, instead of tapering off. With no "old" vs. "new" distinction possible yet (nothing has had time to become "old"), there's no growing pool of easy reuse-candidates to increasingly dilute the rate with - so the rate doesn't decline, it just runs at roughly its ceiling until there's nothing left to onboard. (The two lone stragglers at 4900ms and past 6000ms are presumably just scheduling/GC noise, not part of the core mechanism.)

So both mechanisms are real, but they don't contribute equally - the declining shape in the original `WORK_MS=200` histogram was, at least in part, a genuine artifact of reuse competing with onboarding, not purely the `P`/retake throughput ceiling working alone.

**Caveat: `worker.go` itself spawns goroutines in staggered batches** - 200 at a time, 50ms apart, 10 batches - so all `WORKERS=2000` don't actually exist until ~450-500ms after the container starts. That's a fixed, `WORK_MS`-independent ~500ms of "artificial" spread at the front of every ramp,contributed by the test harness rather than by `sysmon`. It doesn't change the conclusion, though: it's far smaller than either observed ramp, and most of both ramps happens *after* every goroutine already exists. Netting it out actually sharpens the contrast above - 2600ms − 500ms ≈ 2100ms of genuine retake+reuse dynamics for `WORK_MS=200` versus 1300ms − 500ms ≈ 800ms of retake-alone dynamics for `WORK_MS=60000`, a ~2.6x gap rather than the raw ~2x one.
