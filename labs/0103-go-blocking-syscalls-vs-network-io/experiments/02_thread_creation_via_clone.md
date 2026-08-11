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
```
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

The shape is the more interesting part, though - it isn't a smooth, uniform trickle either. Counts swing unevenly (180, then 142, then 122, then back up to 161...) while trending clearly downward over the run: the first several 100ms buckets mostly sit in the 120-180 range, the back half mostly in the 25-75 range, tapering to a thin one-event tail. That decline is itself explained by thread *reuse*: `sysmon`'s retake hands a newly-blocked `P` to *some* M, and only has to `clone()` a fresh one if every existing M is itself already stuck in a syscall. Early on, with almost no Ms yet in existence, nearly every retake needs a brand new thread. As the M population grows toward the ~2000 needed to cover every concurrently-blocked goroutine, a growing fraction of retakes can just reuse an M that's already been created and is between syscalls - so the `clone()` rate decays even though goroutines are blocking and unblocking in their `Nanosleep` loop at a roughly constant rate the entire time.
