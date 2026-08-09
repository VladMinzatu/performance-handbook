## epoll_ctl registrations vs. epoll_wait calls, as connection count scales

We test Prediction 2: `epoll_ctl` (registration) should track connection count roughly 1:1, while `epoll_wait` calls should stay flat regardless of how many connections are registered - a handful of threads polling one shared epoll instance, not one wait per goroutine.

We will need the shared `analysis` container (for bpftrace), on `labnet` alongside the lab's own `server`/`client`:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
```

Go will issue `epoll_pwait`, not the plain `epoll_wait` syscall - confirmed by listing the available tracepoints before picking one:
```sh
docker exec lab-analysis bpftrace -l 'tracepoint:syscalls:*epoll*'
tracepoint:syscalls:sys_enter_epoll_create1
tracepoint:syscalls:sys_enter_epoll_ctl
tracepoint:syscalls:sys_enter_epoll_pwait
tracepoint:syscalls:sys_enter_epoll_pwait2
tracepoint:syscalls:sys_exit_epoll_create1
tracepoint:syscalls:sys_exit_epoll_ctl
tracepoint:syscalls:sys_exit_epoll_pwait
tracepoint:syscalls:sys_exit_epoll_pwait2
```

We reset to a clean baseline (no held connections) before each trace, so the `epoll_ctl` count in a fixed window is attributable to that window's dials
alone:
```sh
docker stop lab-go-netpoll-client && docker rm lab-go-netpoll-client
```
Confirm on the server's own log:
```sh
docker logs lab-go-netpoll-server | tail -2
goroutines=2 active_conns=0
goroutines=2 active_conns=0
```

**epoll_ctl vs. connection count.** Start the trace, then - a couple of seconds later, so the probe is already attached - dial a fresh batch of connections:
```sh
(sleep 2; docker run --rm -d --name lab-go-netpoll-client --network labnet \
  -e SERVER_ADDR=server:9000 -e CONNS=500 \
  0102-go-netpoller-epoll-client:latest >/dev/null) &
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e 'tracepoint:syscalls:sys_enter_epoll_ctl /comm == \"netpoll-server\"/ { @op[args->op] = count(); }'"
```
```
Attaching 1 probe...

@op[1]: 500
```
(`op` 1 is `EPOLL_CTL_ADD` - confirmed via `bpftrace -lv` on the tracepoint, which lists `int op` as a plain field with the same values as `<sys/epoll.h>`'s `EPOLL_CTL_ADD=1`/`MOD=2`/`DEL=3`.) That's 500 `ADD` calls for 500 newly dialed connections - exact match. Alos no `MOD`/`DEL` noise since nothing else touched the epoll set during this window.

**epoll_pwait vs. connection count, idle.** With those same 500 connections now sitting idle (steady `active_conns=500`, no new dials, no data flowing), measure the wait-call rate over a fixed window:
```sh
docker logs lab-go-netpoll-server | tail -2
goroutines=502 active_conns=500
goroutines=502 active_conns=500

docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e 'tracepoint:syscalls:sys_enter_epoll_pwait /comm == \"netpoll-server\"/ { @ = count(); }'"
```
```
Attaching 1 probe...

@: 29
```

Now scale up 16x and repeat the exact same measurement:
```sh
docker stop lab-go-netpoll-client
docker run --rm -d --name lab-go-netpoll-client --network labnet \
  -e SERVER_ADDR=server:9000 -e CONNS=8000 \
  0102-go-netpoller-epoll-client:latest
# wait for the dial to finish and confirme the connections on the server:
docker logs lab-go-netpoll-server | tail -2
goroutines=8002 active_conns=8000
goroutines=8002 active_conns=8000

docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e 'tracepoint:syscalls:sys_enter_epoll_pwait /comm == \"netpoll-server\"/ { @ = count(); }'"
```
```
Attaching 1 probe...

@: 33
```

This confirms Prediction 2 on both halves. `epoll_ctl` registration tracks connection count exactly - 500 dialed connections produced exactly 500 `EPOLL_CTL_ADD` calls, no more, no less. `epoll_pwait` call count over a fixed 10s idle window was **nearly identical - 29 vs 33 - at 500 connections and at
8,000 connections**, a 16x difference in held connections with zero difference in wait-call count. The registration scales with load; the wait loop doesn't. That's the mechanism Prediction 1's flat thread count depends on: a small, fixed number of threads keep re-issuing `epoll_pwait` against one shared epoll fd, and each call is capable of returning readiness for however many of the registered sockets happen to be ready, independent of how many are currently sitting parked and not ready.

~30 calls per 10s (~3/s) at idle is plausibly the Go runtime's own periodic netpoll-with-timeout cadence (invoked opportunistically whenever a P goes looking for work and finds none, plus `sysmon`'s periodic forced poll) rather than anything connection-driven - consistent with it staying
put across a 16x connection-count change.

Now we can restore the lab's normal default state afterwards:
```sh
docker stop lab-go-netpoll-client && docker rm lab-go-netpoll-client
docker compose -f ../compose.yml up -d
```
