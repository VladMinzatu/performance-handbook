## Waking thousands of parked goroutines at once creates zero new threads

We test Prediction 3: triggering the client's `/burst` endpoint - which writes to every held connection at once, making every parked server-side goroutine runnable in a very short window - should not show up as new OS thread creation (`clone`/`clone3` syscalls) on the server, in contrast to what a thread-per-blocking-read design would need to service the same event.

First, let's get a feel for how long a burst actually takes, with the lab's normal default state running (`GOMAXPROCS=2`, `CONNS=2000`):
```sh
time curl -s -X POST http://localhost:8090/burst
{"conns":"2000","max":"124.344847ms","p50":"70.538909ms","p99":"123.129671ms","total":"134.574723ms"}
curl -s -X POST http://localhost:8090/burst  0.00s user 0.01s system 4% cpu 0.184 total
```

So the whole burst - 2000 connections becoming runnable and finishing their round trip - completes in well under a second. Any tracing window of a few seconds will comfortably cover a full burst.

**Idle baseline first.** Confirm the server isn't creating threads on its own, absent any burst, over the same kind of window:
```sh
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e 'tracepoint:syscalls:sys_enter_clone /comm == \"netpoll-server\"/ { @ = count(); }'"
```
```
Attaching 1 probe...

@: 0
```

**Burst, at the default 2000 connections.** Start the trace, then fire the burst two seconds in, so the probe is already attached:
```sh
(sleep 2; curl -s -X POST http://localhost:8090/burst) &
docker exec lab-analysis sh -c "timeout -s INT 8 bpftrace -e 'tracepoint:syscalls:sys_enter_clone /comm == \"netpoll-server\"/ { @ = count(); }'"
wait
```
```
Attaching 1 probe...
{"conns":"2000","max":"128.034461ms","p50":"77.721968ms","p99":"124.636725ms","total":"140.28802ms"}

@: 0
```

**Bigger burst, to push harder.** Scale up to 8000 held connections and repeat, this time also checking the server's real OS thread count immediately before and after:
```sh
docker stop lab-go-netpoll-client && docker rm lab-go-netpoll-client

docker run --rm -d --name lab-go-netpoll-client --network labnet -p 8090:8090 \
  -e SERVER_ADDR=server:9000 -e CONNS=8000 \
  0102-go-netpoller-epoll-client:latest
# wait for the dial to finish and confirm connections on the server:
docker logs lab-go-netpoll-server | tail -2
goroutines=8002 active_conns=8000
goroutines=8002 active_conns=8000

docker exec lab-go-netpoll-server sh -c 'grep Threads /proc/1/status'
Threads:	5

(sleep 2; curl -s -X POST http://localhost:8090/burst) &
docker exec lab-analysis sh -c "timeout -s INT 8 bpftrace -e 'tracepoint:syscalls:sys_enter_clone /comm == \"netpoll-server\"/ { @ = count(); }'"
wait

docker exec lab-go-netpoll-server sh -c 'grep Threads /proc/1/status'
```
```
Attaching 1 probe...
{"conns":"8000","max":"461.458475ms","p50":"239.597659ms","p99":"457.501838ms","total":"492.81965ms"}

@: 0
Threads:	5
```

**Covering the newer `clone3` syscall too**, in case the runtime/kernel combination uses that instead of `clone` (both were listed as available tracepoints before):
```sh
(sleep 2; curl -s -o /dev/null -X POST http://localhost:8090/burst) &
docker exec lab-analysis sh -c "timeout -s INT 8 bpftrace -e 'tracepoint:syscalls:sys_enter_clone /comm == \"netpoll-server\"/ { @[\"clone\"] = count(); } tracepoint:syscalls:sys_enter_clone3 /comm == \"netpoll-server\"/ { @[\"clone3\"] = count(); }'"
wait
```
```
Attaching 2 probes...


```
Both maps stayed empty - no `clone` and no `clone3` events at all.

This confirms Prediction 3 cleanly. Across every burst tested, whether 2000 connections or 8000 connections and checking against both `clone` and `clone3`, the server issued **zero** new-thread syscalls while thousands of parked goroutines went from blocked to runnable to finished in well under
a second. `/proc/1/status`'s `Threads:` count corroborates it directly: still 5, identical before and immediately after the 8000-connection burst - the same count Prediction 1 already found flat across the 100-to-10,000-connection idle range.

This is the concrete difference between "goroutine becomes runnable" and "OS thread gets created." `netpollready` (triggered here by each connection's fd going readable) just moves a parked goroutine onto a P's run queue - existing Ms that are already looking for work (or become free as they finish other goroutines) pick it up. There's no path from "goroutine unparked" to `clone()` in this flow at all; the number of Ms is governed by `GOMAXPROCS` and by genuine blocking-syscall accounting, not
by how many goroutines just became runnable at once. A thread-per-blocking-read server would have no way to service 8000 simultaneously readable sockets without either 8000 threads already sitting there
waiting, or forking that many on the spot - this design needs neither.

Note what this experiment does *not* show: it says nothing about how long those newly-runnable goroutines take to actually get CPU time and finish - only that thread creation isn't part of the cost. The `p50`/`p99`/`max` numbers surfaced above as a side effect (~70-140ms for 2000 connections,
all competing for `GOMAXPROCS=2`) are exactly the queuing-for-a-P question that Prediction 4 covers separately.

Restore the lab's normal default state afterwards:
```sh
docker stop lab-go-netpoll-client
docker compose -f compose.yml up -d
```