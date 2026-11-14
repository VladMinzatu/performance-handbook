## Moving the computation to spawn_blocking

Tests **Prediction 4** (moving the computation to `spawn_blocking` fixes
tokio, and beats Go).

Same load as in the previous experiment: 200 req/s to `/cheap` and 2
`/heavy` requests always in flight, on the same 2 cores. The only change
is in the tokio server: `/heavy` now runs its loop inside
`tokio::task::spawn_blocking` and `.await`s the result. The computation
moves to threads from tokio's blocking pool, and the worker threads only
run the async parts: parsing requests, writing responses and polling
`epoll`. The cores are just as busy as before; the difference is that
the kernel, which can preempt any thread, now shares them between the
computing threads and the workers.

The Go server doesn't change, so its numbers from the previous
experiment are the comparison.

Start the analysis container and the lab, from this lab's directory,
with the tokio server in `blocking` mode and the client pointed at it:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
HEAVY_MODE=blocking TARGET=tokio docker compose -f compose.yml up -d --build --force-recreate
```

### The server's mode

```sh
docker logs lab-go-preempt-tokioserver
```
producing output:
```
workers=2 HEAVY_MS=200 HEAVY_MODE=blocking iters=73100008
```

### The client's view

Wait for at least 5 reports (about 25s):
```sh
docker logs --tail 5 lab-go-preempt-client
```
producing output:
```
2026/11/14 12:34:44 target=http://lab-go-preempt-tokioserver:8080 rate=200/s heavy_conc=2
t=   5s cheap ok=  999 err=  0 p50=400µs    p99=2.8ms    max=35.1ms   | heavy ok= 46 mean=210ms
t=  10s cheap ok= 1000 err=  0 p50=400µs    p99=1.3ms    max=9.7ms    | heavy ok= 50 mean=204ms
t=  15s cheap ok= 1000 err=  0 p50=500µs    p99=3.1ms    max=16.7ms   | heavy ok= 48 mean=207ms
t=  20s cheap ok= 1000 err=  0 p50=400µs    p99=1.9ms    max=30.4ms   | heavy ok= 48 mean=207ms
```


### Gaps between epoll waits

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
@gap_ms: 
[0]                  211 |@@@@@@@@                                            |
[1]                  122 |@@@@                                                |
[2, 4)               567 |@@@@@@@@@@@@@@@@@@@@@@                              |
[4, 8)              1305 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[8, 16)                8 |                                                    |
[16, 32)               1 |                                                    |
[32, 64)               1 |                                                    |


@max_gap_ms: 34
```

### Per-thread CPU

CPU per thread over 5s. Worker and blocking-pool threads are both named
`tokio-rt-worker`; they can be told apart by how busy they are:
```sh
docker exec lab-analysis sh -c 'pidstat -t -p $(pgrep -x tokioserver) 5 1' | grep -E 'TID|Average'
```
producing output:
```
12:35:47      UID      TGID       TID    %usr %system  %guest   %wait    %CPU   CPU  Command
Average:        0      9794         -  197.21    0.00    0.00    0.00  197.21     -  tokioserver
Average:        0         -      9794    0.00    0.00    0.00    0.00    0.00     -  |__tokioserver
Average:        0         -      9905    0.00    0.00    0.00    4.79    0.00     -  |__tokio-rt-worker
Average:        0         -      9906    0.00    0.20    0.00    2.40    0.20     -  |__tokio-rt-worker
Average:        0         -      9907   64.27    0.00    0.00    1.00   64.27     -  |__tokio-rt-worker
Average:        0         -      9908   66.47    0.00    0.00    0.80   66.47     -  |__tokio-rt-worker
Average:        0         -      9927   66.27    0.00    0.00    1.00   66.27     -  |__tokio-rt-worker
```

### Run-queue latency of the tokio threads

The time from a thread being woken up to it actually running on a CPU,
over 10s. Worker threads wake up all the time, from `epoll` when a
request arrives, or when another thread hands them a task. The
computing threads rarely do, since they never wait. So this shows how
long the kernel makes a worker wait for a core that's busy computing.

The filter is on the thread name, not the cgroup: a wakeup is recorded
in the context of whoever does the waking, which is often a network
interrupt rather than a thread in the server's container.
```sh
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e '
tracepoint:sched:sched_wakeup,tracepoint:sched:sched_wakeup_new /args.comm == \"tokio-rt-worker\"/ { @woken[args.pid] = nsecs; }
tracepoint:sched:sched_switch /@woken[args.next_pid]/ {
  @runq_us = hist((nsecs - @woken[args.next_pid]) / 1000);
  @max_runq_us = max((nsecs - @woken[args.next_pid]) / 1000);
  delete(@woken[args.next_pid]);
}
END { clear(@woken); }'"
```
producing output:
```
Attaching 4 probes...


@max_runq_us: 3715
@runq_us: 
[0]                   91 |@@@@@@                                              |
[1]                   23 |@                                                   |
[2, 4)               157 |@@@@@@@@@@                                          |
[4, 8)                29 |@                                                   |
[8, 16)               31 |@@                                                  |
[16, 32)              79 |@@@@@                                               |
[32, 64)              38 |@@                                                  |
[64, 128)            174 |@@@@@@@@@@@                                         |
[128, 256)           291 |@@@@@@@@@@@@@@@@@@@                                 |
[256, 512)           588 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@             |
[512, 1K)            783 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[1K, 2K)              12 |                                                    |
[2K, 4K)               9 |                                                    |
```
