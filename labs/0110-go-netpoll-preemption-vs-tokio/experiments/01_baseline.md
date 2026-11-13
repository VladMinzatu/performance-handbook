## Baseline without CPU-bound work

Tests **Prediction 1** (without CPU-bound work, both runtimes are
equally fast). With no `/heavy` traffic, the only difference between
the two servers is the language, the HTTP stack and the runtime. If
`/cheap` latency is about the same on both, the differences in the
later experiments come from how each runtime handles the computation.

This run also records two reference points for the later experiments:
how often each server calls `epoll_wait` when nothing is stalling it,
and what its threads look like when they're mostly idle.

Start the analysis container and the lab, from this lab's directory,
with the client sending only `/cheap` traffic to the Go server:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
HEAVY_CONC=0 docker compose -f compose.yml up -d --build --force-recreate
```
`--force-recreate` starts both servers fresh, so nothing from an earlier
run carries over.

### Both servers do the same work per `/heavy` call

Each server calibrates its loop at startup. The loop counts should be
within a few percent of each other, so a `/heavy` call is the same
amount of work on both servers.

```sh
docker logs lab-go-preempt-goserver
docker logs lab-go-preempt-tokioserver
```
producing output:
```
2026/11/13 11:08:16 GOMAXPROCS=2 HEAVY_MS=200 iters=73276459

workers=2 HEAVY_MS=200 HEAVY_MODE=inline iters=72970624
```


### Go: the client's view

Wait for a few 5s reports:
```sh
docker logs --tail 5 lab-go-preempt-client
```
producing output:
```
t=  35s cheap ok= 1000 err=  0 p50=400µs    p99=1ms      max=5.4ms    | heavy ok=  0 mean=0s
t=  40s cheap ok= 1000 err=  0 p50=400µs    p99=1ms      max=7.9ms    | heavy ok=  0 mean=0s
t=  45s cheap ok= 1000 err=  0 p50=400µs    p99=1.1ms    max=11.7ms   | heavy ok=  0 mean=0s
t=  50s cheap ok= 1000 err=  0 p50=400µs    p99=900µs    max=1.6ms    | heavy ok=  0 mean=0s
t=  55s cheap ok= 1000 err=  0 p50=400µs    p99=800µs    max=17.1ms   | heavy ok=  0 mean=0s
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
[0]                 3597 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[1]                    6 |                                                    |
[2, 4)               406 |@@@@@                                               |
[4, 8)              1554 |@@@@@@@@@@@@@@@@@@@@@@                              |
[8, 16)                6 |                                                    |
[16, 32)               2 |                                                    |

@max_gap_ms: 26
```

### Go: per-thread CPU

CPU per thread over 5s:
```sh
docker exec lab-analysis sh -c 'pidstat -t -p $(pgrep -x goserver) 5 1' | grep -E 'TID|Average'
```
producing output:
```
11:10:27      UID      TGID       TID    %usr %system  %guest   %wait    %CPU   CPU  Command
Average:        0      8260         -    2.00    2.00    0.00    0.20    3.99     -  goserver
Average:        0         -      8260    0.60    0.60    0.00    0.20    1.20     -  |__goserver
Average:        0         -      8304    0.20    0.40    0.00    0.80    0.60     -  |__goserver
Average:        0         -      8305    0.00    0.00    0.00    0.00    0.00     -  |__goserver
Average:        0         -      8306    0.40    0.60    0.00    0.20    1.00     -  |__goserver
Average:        0         -      8367    0.80    0.40    0.00    0.40    1.20     -  |__goserver
```

### tokio: the client's view

Switch the client to the tokio server. `--no-deps` keeps compose from
recreating the servers:
```sh
TARGET=tokio HEAVY_CONC=0 docker compose -f compose.yml up -d --no-deps client
```
Wait for a few 5s reports:
```sh
docker logs --tail 5 lab-go-preempt-client
```
producing output:
```
t=   5s cheap ok=  998 err=  0 p50=300µs    p99=500µs    max=2.7ms    | heavy ok=  0 mean=0s
t=  10s cheap ok= 1001 err=  0 p50=300µs    p99=1.2ms    max=5.7ms    | heavy ok=  0 mean=0sO
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
[0]                   69 |@@                                                  |
[1]                    7 |                                                    |
[2, 4)               444 |@@@@@@@@@@@@@@@                                     |
[4, 8)              1519 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[8, 16)                5 |                                                    |
[16, 32)               1 |                                                    |

@max_gap_ms: 19
```


### tokio: per-thread CPU

CPU per thread over 5s:
```sh
docker exec lab-analysis sh -c 'pidstat -t -p $(pgrep -x tokioserver) 5 1' | grep -E 'TID|Average'
```
producing output:
```
11:13:03      UID      TGID       TID    %usr %system  %guest   %wait    %CPU   CPU  Command
Average:        0      8265         -    0.60    1.00    0.00    0.00    1.60     -  tokioserver
Average:        0         -      8265    0.00    0.00    0.00    0.00    0.00     -  |__tokioserver
Average:        0         -      8364    0.60    1.00    0.00    0.00    1.60     -  |__tokio-rt-worker
Average:        0         -      8365    0.20    0.00    0.00    0.00    0.20     -  |__tokio-rt-worker
```
