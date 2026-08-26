## Pure hand-off coordination: channel ring vs. mutex-and-condvar ring

Tests Prediction 2: for pure coordinated hand-off (no shared state, just "whose turn is it"), a channel ring should beat a `sync.Mutex` + `sync.Cond` ring - the opposite direction from experiment 01, same underlying reason (a channel send wakes exactly the one goroutine next in line; `Cond.Broadcast` wakes every waiter, most of whom immediately find it isn't their turn).

Make sure the lab's default setup is running:
```sh
docker compose -f compose.yml up -d --build
```

Confirm both started correctly:
```sh
docker logs lab-go-pingpong-mutex | head -1
mech=mutex participants=4 GOMAXPROCS=8
...
docker logs lab-go-pingpong-channel | head -1
mech=channel participants=4 GOMAXPROCS=8
```

After a bit, let's compare hand-off throughput:
```sh
echo "mutex:";   docker logs lab-go-pingpong-mutex   | tail -3
mutex:
handoffs/sec=3072753 goroutines=5
handoffs/sec=3144073 goroutines=5
handoffs/sec=3209657 goroutines=5


echo "channel:"; docker logs lab-go-pingpong-channel | tail -3
channel:
handoffs/sec=9813196 goroutines=5
handoffs/sec=9482521 goroutines=5
handoffs/sec=9303259 goroutines=5
```

**Channel ~3.0x faster than mutex** (avg 9.53M vs 3.14M handoffs/sec) - confirms Prediction 2.

We cab also compare CPU usage over the same window (not what Prediction 2 is about, but cheap to grab alongside experiment 01's counter-workload CPU numbers for a fuller picture):
```sh
docker stats --no-stream lab-go-pingpong-mutex lab-go-pingpong-channel
CONTAINER ID   NAME                      CPU %     MEM USAGE / LIMIT     MEM %     NET I/O         BLOCK I/O     PIDS
6b4883d36762   lab-go-pingpong-mutex     98.64%    2.445MiB / 11.74GiB   0.02%     1.27kB / 126B   811kB / 0B    9
c97a5e0f1553   lab-go-pingpong-channel   95.23%    2.27MiB / 11.74GiB    0.02%     1.51kB / 126B   3.67MB / 0B   8
```
**Unlike experiment 01, CPU-per-handoff is *not* the same here**: ~31.4% CPU per million handoffs for mutex vs. ~10.0 for channel - mutex costs ~3.1x more CPU per handoff, matching its throughput deficit almost exactly. Fits the `Broadcast` theory: at the default `PARTICIPANTS=4`, every hand-off wakes 4 goroutines for 1 to proceed - 3 wasted wakeups each time, a real cost the channel ring never pays.


This doubles as most of the setup for Prediction 4 (stretch) - if the
`Cond.Broadcast` thundering herd is real, `pingpong-mutex`'s
`handoffs/sec` should degrade faster than `pingpong-channel`'s as
`PARTICIPANTS` grows, since every hand-off wakes *all* of them either
way, and that wasted-wakeup cost scales with ring size. Both
`pingpong-mutex` and `pingpong-channel` build from the same `pingpong`
target, so either image tag works for both modes:
```sh
for P in 2 4 8 16 32 64; do
  for M in mutex channel; do
    docker stop lab-go-pingpong-${M} >/dev/null 2>&1
    docker rm lab-go-pingpong-${M} >/dev/null 2>&1
    docker run -d --name lab-go-pingpong-${M} \
      -e MECH=${M} -e PARTICIPANTS=$P \
      0105-go-locking-vs-channels-pingpong-mutex:latest >/dev/null
    sleep 4
    HANDOFFS=$(docker logs lab-go-pingpong-${M} | tail -1)
    echo "PARTICIPANTS=$P MECH=${M}: $HANDOFFS"
  done
done
```
As a table, with the channel/mutex ratio at each point:

| PARTICIPANTS | mutex handoffs/sec | channel handoffs/sec | ratio |
|---|---|---|---|
| 2 | 11,748,818 | 11,768,776 | 1.00x |
| 4 | 4,683,661 | 11,833,828 | 2.53x |
| 8 | 2,491,915 | 11,620,060 | 4.66x |
| 16 | 1,082,614 | 11,519,402 | 10.64x |
| 32 | 554,330 | 11,400,237 | 20.57x |
| 64 | 277,245 | 10,619,681 | 38.30x |

**Prediction 4 confirmed, cleanly.** Tied at `PARTICIPANTS=2` (only one other goroutine to wake either way), then the ratio blows up - essentially doubling every time `PARTICIPANTS` doubles. Channel stays flat (~11-12M) regardless of ring size, exactly as expected (always wakes exactly one). Mutex throughput roughly *halves* every doubling (4.68M → 2.49M → 1.08M → 554K → 277K) - a clean 1/`PARTICIPANTS` decay, exactly what `Broadcast` waking everyone predicts.

And to clean up at the end:
```sh
docker compose -f compose.yml down
```
