## Does the ping-pong ring actually reach futex?

Sustained contention on a shared resource doesn't necessarily reach the kernel - if there's always some other runnable goroutine for an idle OS thread to pick up, a thread never has a reason to actually park itself at the OS level, and `futex` never gets called. The ping-pong ring is a useful contrast case: only **one** goroutine is ever runnable at a time, across the whole ring, by construction, regardless of the number of `PARTICIPANTS`. With more OS threads available than there is possible concurrent work, several of them should have genuinely nothing to do - which is exactly the condition that should push a thread into a real, kernel-visible `futex(FUTEX_WAIT)`. 

This experiment checks whether that's true, and whether `mutex` and `channel` differ in the rate.

First, start from the lab's default setup and have the shared analysis container running:
```sh
docker compose -f compose.yml up -d --build
docker compose -f ../tools/analysis/compose.yml up -d --build
```

`futex` op counts for `pingpong-mutex`, over a 10-second window:
```sh
CID=$(docker inspect --format '{{.Id}}' lab-go-pingpong-mutex)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e 'tracepoint:syscalls:sys_enter_futex /cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${CID}\")/ { @[args.op] = count(); }'"
```
which produces output:
```sh
Attaching 1 probe...


@[128]: 26220
@[129]: 34640
```
`args.op` bundles the operation with flag bits - `128` is `FUTEX_WAIT` (`0`) with `FUTEX_PRIVATE_FLAG` (`128`) set, `129` is `FUTEX_WAKE` (`1`) with the same flag - so this is one goroutine parking, another one later waking it up, as expected.

Real `futex` activity, confirmed - unlike the counter workload, this ring genuinely reaches the kernel.

Same thing for `pingpong-channel`:
```sh
CID=$(docker inspect --format '{{.Id}}' lab-go-pingpong-channel)
docker exec lab-analysis sh -c "timeout -s INT 10 bpftrace -e 'tracepoint:syscalls:sys_enter_futex /cgroup == cgroupid(\"/host/sys/fs/cgroup/docker/${CID}\")/ { @[args.op] = count(); }'"
```
which produces output:
```sh
Attaching 1 probe...


@[128]: 36830
@[129]: 42173
```
Higher raw counts than mutex (79,003 vs 60,860 total) - but channel also does far more handoffs in the same window, so raw counts alone are misleading here; needs normalizing.

To make sense of the counts, grab each container's `handoffs/sec` from around the same time, so the futex rate can be normalized per handoff rather than just compared as raw counts (`mutex` and `channel` produce very different total handoffs, so raw counts alone aren't a fair comparison - see experiment 02):
```sh
docker logs lab-go-pingpong-mutex   | tail -3
handoffs/sec=3053143 goroutines=5
handoffs/sec=3151014 goroutines=5
handoffs/sec=3071237 goroutines=5

docker logs lab-go-pingpong-channel | tail -3
handoffs/sec=8682196 goroutines=5
handoffs/sec=8566770 goroutines=5
handoffs/sec=8116958 goroutines=5
```
Normalized (futex events per million handoffs): mutex ≈ 1,968, channel ≈ 934 - **mutex costs about 2.1x more futex activity per handoff**, the opposite of what the raw counts suggested. Same direction as the CPU finding in experiment 02: the `Broadcast` thundering herd doesn't just burn CPU, it also correlates with more OS-level thread parking/waking per unit of actual progress.

And we can clean up at the end:
```sh
docker compose -f compose.yml down
```
