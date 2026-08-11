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

