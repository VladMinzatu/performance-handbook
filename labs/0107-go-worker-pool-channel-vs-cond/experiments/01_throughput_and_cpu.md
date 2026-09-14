## Bounded-queue throughput: buffered channel vs. two-condvar queue

Tests Prediction 1 (bulk throughput should be similar between the two mechanisms, since neither has a structural source of wasted wakeups).

The two implementations are deliberately matched: same `PRODUCERS`, `CONSUMERS` and `QUEUE_CAP`, both correctly bounded, both waking exactly one goroutine per item transferred. If Prediction 1 holds, the remaining difference should be small. If it doesn't, the size and *shape* of the gap across a contention sweep is what says where the extra cost actually lives.

Build and start the lab's default setup:
```sh
docker compose -f compose.yml up -d --build
```

Confirm both started correctly:
```sh
docker logs lab-go-workerpool-channel | head -1
mech=channel producers=8 consumers=8 queue_cap=16 GOMAXPROCS=8


docker logs lab-go-workerpool-cond    | head -1
mech=cond producers=8 consumers=8 queue_cap=16 GOMAXPROCS=8
```

Next, let them run for a few seconds to reach steady state, then compare throughput. Both `produced/sec` and `consumed/sec` are printed and they should track each other closely within each container (a correctly bounded queue can't let producers run away from consumers), so the pair is also a sanity check that neither implementation is quietly broken:
```sh
echo "channel:"; docker logs lab-go-workerpool-channel | tail -3
produced/sec=4937743 consumed/sec=4937748 goroutines=17
produced/sec=5426651 consumed/sec=5426637 goroutines=17
produced/sec=6310491 consumed/sec=6310487 goroutines=17

...

echo "cond:";    docker logs lab-go-workerpool-cond    | tail -3
produced/sec=5022942 consumed/sec=5022928 goroutines=17
produced/sec=4752006 consumed/sec=4752018 goroutines=17
produced/sec=4938534 consumed/sec=4938522 goroutines=17
```
Produced/consumed track each other within each container, so both implementations are correctly bounded. Throughput itself is close - low 5M/s for `channel`, high 4M/s for `cond` - well within the run-to-run noise of a single 3-second sample. At this default depth (`QUEUE_CAP=16`, 8 producers, 8 consumers), **Prediction 1 holds**: no large gap.

Next, compare CPU cost over the same window. This matters independently of throughput: if one mechanism reaches the same items/sec while burning noticeably more CPU, the cost is real but hidden by there being spare cores to absorb it:
```sh
docker stats --no-stream lab-go-workerpool-channel lab-go-workerpool-cond
CONTAINER ID   NAME                        CPU %     MEM USAGE / LIMIT     MEM %     NET I/O         BLOCK I/O    PIDS
ba088aeb91cf   lab-go-workerpool-channel   279.20%   3.379MiB / 11.74GiB   0.03%     1.84kB / 126B   6MB / 0B     10
ad9910eee608   lab-go-workerpool-cond      276.88%   2.469MiB / 11.74GiB   0.02%     1.58kB / 126B   922kB / 0B   12
```
CPU% is essentially identical (both saturating ~2.8 of the container's 8 cores). Combined with the near-equal throughput above, per-item CPU cost is also comparable at this depth - no hidden overhead the raw CPU% alone would have masked. The two-condvar design's disadvantage, if any, isn't visible yet; it needs the sweep below to show up.

### Contention sweep

A single data point at one queue depth doesn't separate "these mechanisms cost the same" from "this particular configuration happens not to block much." The amount of *blocking* is what Prediction 2 and 3 are really about, and blocking is governed by how often the queue sits at full or empty - which is what `QUEUE_CAP` controls relative to the number of producers and consumers.

Sweep `QUEUE_CAP` from deep (rarely full or empty, so most operations complete without ever parking a goroutine) to shallow (the queue spends most of its time at a boundary, so nearly every operation blocks). Both services build the same image, so one tag serves both mechanisms - `MECH` is just an env var:
```sh
IMG=0107-go-worker-pool-channel-vs-cond-workerpool-channel:latest
for Q in 1 2 8 32 128 1024; do
  for M in channel cond; do
    docker rm -f lab-go-workerpool-sweep >/dev/null 2>&1
    docker run -d --name lab-go-workerpool-sweep \
      -e MECH=${M} -e PRODUCERS=8 -e CONSUMERS=8 -e QUEUE_CAP=$Q \
      $IMG >/dev/null
    sleep 5
    echo "QUEUE_CAP=$Q MECH=${M}: $(docker logs lab-go-workerpool-sweep | tail -1)"
  done
done
docker rm -f lab-go-workerpool-sweep >/dev/null 2>&1
```
Which produces the output:
```sh
QUEUE_CAP=1 MECH=channel: produced/sec=4005638 consumed/sec=4005637 goroutines=17
QUEUE_CAP=1 MECH=cond: produced/sec=3812053 consumed/sec=3812053 goroutines=17
QUEUE_CAP=2 MECH=channel: produced/sec=3461343 consumed/sec=3461345 goroutines=17
QUEUE_CAP=2 MECH=cond: produced/sec=2466098 consumed/sec=2466100 goroutines=17
QUEUE_CAP=8 MECH=channel: produced/sec=5702674 consumed/sec=5702671 goroutines=17
QUEUE_CAP=8 MECH=cond: produced/sec=3185013 consumed/sec=3185016 goroutines=17
QUEUE_CAP=32 MECH=channel: produced/sec=5325472 consumed/sec=5325507 goroutines=17
QUEUE_CAP=32 MECH=cond: produced/sec=3531472 consumed/sec=3531460 goroutines=17
QUEUE_CAP=128 MECH=channel: produced/sec=5848148 consumed/sec=5848196 goroutines=17
QUEUE_CAP=128 MECH=cond: produced/sec=4004503 consumed/sec=4004375 goroutines=17
QUEUE_CAP=1024 MECH=channel: produced/sec=4726483 consumed/sec=4726488 goroutines=17
QUEUE_CAP=1024 MECH=cond: produced/sec=4580770 consumed/sec=4581753 goroutines=17
```

Same data, as a table, with the channel/cond ratio at each depth:

| QUEUE_CAP | channel produced/sec | cond produced/sec | ratio |
|---|---|---|---|
| 1 | 4,005,638 | 3,812,053 | 1.05x |
| 2 | 3,461,343 | 2,466,098 | 1.40x |
| 8 | 5,702,674 | 3,185,013 | 1.79x |
| 32 | 5,325,472 | 3,531,472 | 1.51x |
| 128 | 5,848,148 | 4,004,503 | 1.46x |
| 1024 | 4,726,483 | 4,580,770 | 1.03x |

`channel` leads at every depth, but **not in the clean monotonic shape Prediction 2 expected** ("gap widens as the queue gets shallower"). Both extremes (`QUEUE_CAP=1` and `QUEUE_CAP=1024`) are near parity (~1.05x), while the middle of the sweep (`QUEUE_CAP=8`-`128`) shows the largest gap (1.46x-1.79x). A plausible read: at `QUEUE_CAP=1` the queue behaves like an unbuffered rendezvous for *both* mechanisms - nearly every op blocks for `channel` too, so there's little relative advantage left to have. At `QUEUE_CAP=1024` neither mechanism blocks often, so both are on their cheap uncontended path and the gap shrinks from the other direction. The mid-range is where `channel` gets to complete most ops without blocking while `cond` still blocks often enough for the extra mutex round-trip to matter.

Next, we sweep the worker count at a fixed, shallow queue depth, so blocking stays frequent while the number of goroutines competing to be woken grows. This is the axis that would expose a per-waiter scaling cost if one exists - the careful two-condvar design is specifically supposed *not* to have one, since `Signal` wakes a single waiter regardless of how many are queued:
```sh
IMG=0107-go-worker-pool-channel-vs-cond-workerpool-channel:latest
for W in 1 2 4 8 16 32; do
  for M in channel cond; do
    docker rm -f lab-go-workerpool-sweep >/dev/null 2>&1
    docker run -d --name lab-go-workerpool-sweep \
      -e MECH=${M} -e PRODUCERS=$W -e CONSUMERS=$W -e QUEUE_CAP=4 \
      $IMG >/dev/null
    sleep 5
    echo "WORKERS=$W MECH=${M}: $(docker logs lab-go-workerpool-sweep | tail -1)"
  done
done
docker rm -f lab-go-workerpool-sweep >/dev/null 2>&1
```
Which produces the output:
```sh
WORKERS=1 MECH=channel: produced/sec=13806236 consumed/sec=13806236 goroutines=3
WORKERS=1 MECH=cond: produced/sec=10678070 consumed/sec=10678066 goroutines=3
WORKERS=2 MECH=channel: produced/sec=13502808 consumed/sec=13502809 goroutines=5
WORKERS=2 MECH=cond: produced/sec=8961611 consumed/sec=8961615 goroutines=5
WORKERS=4 MECH=channel: produced/sec=10895941 consumed/sec=10895946 goroutines=9
WORKERS=4 MECH=cond: produced/sec=5224152 consumed/sec=5224144 goroutines=9
WORKERS=8 MECH=channel: produced/sec=4218366 consumed/sec=4218372 goroutines=17
WORKERS=8 MECH=cond: produced/sec=2589810 consumed/sec=2589815 goroutines=17
WORKERS=16 MECH=channel: produced/sec=4235180 consumed/sec=4235184 goroutines=33
WORKERS=16 MECH=cond: produced/sec=2505835 consumed/sec=2505836 goroutines=33
WORKERS=32 MECH=channel: produced/sec=3739912 consumed/sec=3739911 goroutines=65
WORKERS=32 MECH=cond: produced/sec=2359832 consumed/sec=2359832 goroutines=65
```

Same data, as a table:

| WORKERS | channel produced/sec | cond produced/sec | ratio |
|---|---|---|---|
| 1 | 13,806,236 | 10,678,070 | 1.29x |
| 2 | 13,502,808 | 8,961,611 | 1.51x |
| 4 | 10,895,941 | 5,224,152 | 2.09x |
| 8 | 4,218,366 | 2,589,810 | 1.63x |
| 16 | 4,235,180 | 2,505,835 | 1.69x |
| 32 | 3,739,912 | 2,359,832 | 1.59x |

The gap opens up fast going from uncontended (`WORKERS=1`, 1.29x) to first real contention (`WORKERS=4`, 2.09x - the peak of the sweep), then **settles to a roughly flat ~1.6-1.7x floor from `WORKERS=8` on**, instead of continuing to widen as more goroutines pile up. That plateau is the important result: it's consistent with the two-condvar design actually delivering on its "no thundering herd" premise - if `Signal` were losing its target and waking extra waiters, or if contention cost scaled with the number of blocked goroutines, the ratio should keep climbing past `WORKERS=8`, the way `Broadcast`-based designs do. It doesn't. The `WORKERS=4`→`8` drop in absolute throughput for *both* mechanisms (channel: 10.9M→4.2M) lines up with `GOMAXPROCS=8` - once producers+consumers exceed available cores, both saturate and the picture shifts from "how much parking costs" to "how much CPU is left to share," which is a general scheduling effect and not specific to either mechanism.

Clean up when done:
```sh
docker compose -f compose.yml down
```
