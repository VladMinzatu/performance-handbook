## Lock traffic per item: does the cond queue really touch the runtime lock more?

Tests Prediction 2 directly. Experiment 01's contention sweep showed `channel` ahead of `cond` at shallow queue depths, consistent with Prediction 2's "extra mutex round-trip" story - but that was inferred from a throughput gap, not measured directly. Here, we trace the actual runtime-level lock acquisitions each mechanism makes, normalized per item transferred, at a fixed contended configuration.

Both mechanisms end up calling the Go runtime's own low-level lock (`runtime.lock2`/`runtime.unlock2`) whenever something actually blocks: a channel op locks the channel's internal `hchan.lock` for the duration of every send/receive, blocked or not; a blocked `Cond.Wait()`/`Signal()` pair separately locks the contended-parking machinery underneath `sync.Mutex` and the `Cond`'s own internal wait-list lock. Counting `runtime.lock2` hits per container captures all of that machinery in one number, without having to assume in advance which specific internal structure is responsible.

Start both mechanisms at a shallow, contended queue depth - reusing the config from experiment 01's worker sweep that settled into a stable ~1.6x-1.7x throughput gap:
```sh
docker rm -f lab-go-workerpool-channel lab-go-workerpool-cond >/dev/null 2>&1
IMG=0107-go-worker-pool-channel-vs-cond-workerpool-channel:latest
docker run -d --name lab-go-workerpool-channel \
  -e MECH=channel -e PRODUCERS=8 -e CONSUMERS=8 -e QUEUE_CAP=4 $IMG
docker run -d --name lab-go-workerpool-cond \
  -e MECH=cond -e PRODUCERS=8 -e CONSUMERS=8 -e QUEUE_CAP=4 $IMG
```

Start the shared analysis container, if it isn't already running:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
```

Resolve each container's binary path as seen from `analysis`. Each container has its own filesystem, so the path via `/proc/<pid>/root/...` is already unique per container - unlike a probe on a system-wide tracepoint, no cgroup filter is needed to scope it:
```sh
CHAN_PID=$(docker inspect --format '{{.State.Pid}}' lab-go-workerpool-channel)
COND_PID=$(docker inspect --format '{{.State.Pid}}' lab-go-workerpool-cond)
CHAN_BIN=/proc/${CHAN_PID}/root/usr/local/bin/workerpool
COND_BIN=/proc/${COND_PID}/root/usr/local/bin/workerpool
echo "channel: $CHAN_BIN"
echo "cond:    $COND_BIN"
```

Sanity-check the probe actually resolves before trusting a count from it:
```sh
docker exec lab-analysis bpftrace -l "uprobe:${CHAN_BIN}:runtime.lock2"
uprobe:/proc/5987/root/usr/local/bin/workerpool:runtime.lock2

...

docker exec lab-analysis bpftrace -l "uprobe:${COND_BIN}:runtime.lock2"
uprobe:/proc/6034/root/usr/local/bin/workerpool:runtime.lock2
```

Count `runtime.lock2` hits over a fixed window for each container:
```sh
docker exec lab-analysis timeout -s INT 10 bpftrace -e \
  "uprobe:${CHAN_BIN}:runtime.lock2 { @locks = count(); }"

Attaching 1 probe...


@locks: 46082132
```

```sh
docker exec lab-analysis timeout -s INT 10 bpftrace -e \
  "uprobe:${COND_BIN}:runtime.lock2 { @locks = count(); }"

Attaching 1 probe...


@locks: 47229454
```
`channel`: 46,082,132 hits. `cond`: 47,229,454 hits - almost the same raw count, nowhere near the 2x Prediction 2 expected. But raw counts aren't the comparison that matters here (the two mechanisms don't process the same number of items in the window) - normalizing against throughput below is what actually tests the prediction.

Next, grab each container's throughput from roughly the same window, to normalize lock count per item transferred rather than compare raw counts - the two mechanisms won't necessarily process the same number of items in 10 seconds:
```sh
docker logs lab-go-workerpool-channel | tail -3
produced/sec=4833569 consumed/sec=4833564 goroutines=17
produced/sec=4594621 consumed/sec=4594623 goroutines=17
produced/sec=4358870 consumed/sec=4358877 goroutines=17


docker logs lab-go-workerpool-cond    | tail -3
produced/sec=3478476 consumed/sec=3478478 goroutines=17
produced/sec=3467870 consumed/sec=3467870 goroutines=17
produced/sec=3671325 consumed/sec=3671324 goroutines=17
```

Average `consumed/sec` over these three samples: `channel` ≈ 4,595,688,
`cond` ≈ 3,539,224. Scaling each to the ~10s trace window and dividing
into the lock counts above gives lock hits per item transferred:

| mechanism | locks (10s) | items (10s, est.) | locks/item |
|---|---|---|---|
| channel | 46,082,132 | ~45,956,880 | ~1.00x |
| cond | 47,229,454 | ~35,392,240 | ~1.33x |

`channel` lands almost exactly on **one `lock2` pair per item** - a clean result that also validates the technique: a direct-handoff channel send/receive completes the whole transfer inside a single `hchan.lock` critical section, so one lock op per item is exactly what the mechanism should produce. `cond` comes in around **1.33 lock ops per item, not the ~2x Prediction 2 predicted**. The relative gap (cond ÷ channel ≈ 1.33x) is also smaller than the ~1.6x-1.7x *throughput* gap experiment 01 measured at this same configuration (`WORKERS=8`, `QUEUE_CAP=4`) - and this run's own throughput ratio (4,595,688 / 3,539,224 ≈ 1.30x) is itself lower than experiment 01's untraced 1.63x at that point. That discrepancy is a flag, not just noise: both containers were sustaining tens of millions of probe hits over 10 seconds, and `bpftrace` uprobes carry real per-hit overhead at that frequency - the trace itself was very likely perturbing throughput, and not necessarily by the same amount for both mechanisms, so the absolute ratios here should be read as directional, not precise.


Clean up:
```sh
docker rm -f lab-go-workerpool-channel lab-go-workerpool-cond
docker compose -f compose.yml down
```
