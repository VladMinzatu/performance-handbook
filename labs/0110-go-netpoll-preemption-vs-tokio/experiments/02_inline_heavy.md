## Inline computation: tokio stalls, Go doesn't

Tests **Prediction 2** (with the computation inline in the handler,
tokio's fast requests wait for the slow ones, and Go's don't) and
**Prediction 3** (from the outside, the stall shows up as gaps in
`epoll_wait`).

The client keeps sending 200 req/s to `/cheap`, and now also keeps 2
`/heavy` requests in flight, each computing for about 200ms inline in
the handler. With 2 workers per server, that's one computation per
worker at all times. Both servers run the same handler code: the only
difference is whether the runtime can interrupt it.

Start the analysis container and the lab with its defaults, from this
lab's directory. The client starts against the Go server:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
docker compose -f compose.yml up -d --build --force-recreate
```
`--force-recreate` starts both servers fresh, with the tokio server in
its default `HEAVY_MODE=inline`.

### Go: the client's view

Wait for a few 5s reports:
```sh
docker logs --tail 5 lab-go-preempt-client
```
producing output:
```
2026/11/13 18:10:50 target=http://lab-go-preempt-goserver:8080 rate=200/s heavy_conc=2
t=   5s cheap ok=  996 err=  0 p50=8.7ms    p99=30.1ms   max=51.9ms   | heavy ok= 48 mean=206ms
```

### Go: gaps between epoll waits

```sh
SRV=$(docker inspect --format '{{.Id}}' lab-go-preempt-goserver)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e '
tracepoint:syscalls:sys_enter_epoll_*wait*
/cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${SRV}\")/ {
  if (@last) { @gap_ms = hist((nsecs - @last) / 1000000); @max_gap_ms = max((nsecs - @last) / 1000000); }
  @last = nsecs;
}
END { clear(@last); }'"
```
producing output:
```
Attaching 3 probes...

@gap_ms: 
[0]                  256 |@@@@@@@@@@@@@@                                      |
[1]                    5 |                                                    |
[2, 4)                 6 |                                                    |
[4, 8)                44 |@@                                                  |
[8, 16)              894 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[16, 32)              22 |@                                                   |
[32, 64)               0 |                                                    |
[64, 128)              1 |                                                    |
```

### Go: preemption signals

Go preempts a goroutine that has run for more than 10ms by sending its
thread a `SIGURG` (signal 23). Counting the `SIGURG`s generated in the
server's cgroup over 10s shows how often the runtime interrupts the
computation:
```sh
SRV=$(docker inspect --format '{{.Id}}' lab-go-preempt-goserver)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e '
tracepoint:signal:signal_generate
/args.sig == 23 && cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${SRV}\")/ { @sigurg[args.comm] = count(); }'"
```
producing output:
```
Attaching 1 probe...

@sigurg[goserver]: 986
```

### Go: per-thread CPU

CPU per thread over 5s:
```sh
docker exec lab-analysis sh -c 'pidstat -t -p $(pgrep -x goserver) 5 1' | grep -E 'TID|Average'
```
producing output:
```
18:12:33      UID      TGID       TID    %usr %system  %guest   %wait    %CPU   CPU  Command
Average:        0      9032         -  200.00    0.00    0.00    0.40  200.00     -  goserver
Average:        0         -      9032   99.80    0.00    0.00    0.40   99.80     -  |__goserver
Average:        0         -      9067    0.00    0.00    0.00    0.20    0.00     -  |__goserver
Average:        0         -      9068    0.00    0.00    0.00    0.00    0.00     -  |__goserver
Average:        0         -      9069  100.00    0.00    0.00    0.00  100.00     -  |__goserver
```

### tokio: the client's view

Switch the client to the tokio server. `--no-deps` keeps compose from
recreating the servers:
```sh
TARGET=tokio docker compose -f compose.yml up -d --no-deps client
```
Wait for a few 5s reports:
```sh
docker logs --tail 5 lab-go-preempt-client
```
producing output:
```
2026/11/13 18:13:04 target=http://lab-go-preempt-tokioserver:8080 rate=200/s heavy_conc=2
```

### tokio: gaps between epoll waits

```sh
SRV=$(docker inspect --format '{{.Id}}' lab-go-preempt-tokioserver)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e '
tracepoint:syscalls:sys_enter_epoll_*wait*
/cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${SRV}\")/ {
  if (@last) { @gap_ms = hist((nsecs - @last) / 1000000); @max_gap_ms = max((nsecs - @last) / 1000000); }
  @last = nsecs;
}
END { clear(@last); }'"
```
producing output:
```
Attaching 3 probes...

@gap_ms: 
[0]                   81 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[1]                    5 |@@@                                                 |
[2, 4)                 8 |@@@@@                                               |
[4, 8)                 1 |                                                    |
[8, 16)                0 |                                                    |
[16, 32)               0 |                                                    |
[32, 64)               0 |                                                    |
[64, 128)              0 |                                                    |
[128, 256)            48 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                      |

@max_gap_ms: 224
```

### tokio: preemption signals

The same count for the tokio server. Nothing in tokio interrupts a
running task, so this should print no `@sigurg` map at all:
```sh
SRV=$(docker inspect --format '{{.Id}}' lab-go-preempt-tokioserver)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e '
tracepoint:signal:signal_generate
/args.sig == 23 && cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${SRV}\")/ { @sigurg[args.comm] = count(); }'"
```
producing output:
```
Attaching 1 probe...

```

### tokio: per-thread CPU

CPU per thread over 5s:
```sh
docker exec lab-analysis sh -c 'pidstat -t -p $(pgrep -x tokioserver) 5 1' | grep -E 'TID|Average'
```
producing output:
```
18:14:19      UID      TGID       TID    %usr %system  %guest   %wait    %CPU   CPU  Command
Average:        0      9011         -  199.80    0.20    0.00    0.00  200.00     -  tokioserver
Average:        0         -      9011    0.00    0.00    0.00    0.00    0.00     -  |__tokioserver
Average:        0         -      9128  100.00    0.00    0.00    0.00  100.00     -  |__tokio-rt-worker
Average:        0         -      9129  100.00    0.00    0.00    0.00  100.00     -  |__tokio-rt-worker
```
