## Shared-counter coordination: throughput and CPU cost

Tests Prediction 1 (a direct mutex should beat a channel-mediated equivalent for protecting shared state) and Prediction 3 (the difference should show up in CPU usage, not just throughput) together, since both
just need `counter-mutex`/`counter-channel` running at their defaults.

Build and start the lab's default setup:
```sh
docker compose -f compose.yml up -d --build
```

Confirm both started correctly:
```sh
docker logs lab-go-counter-mutex | head -1
mech=mutex workers=32 GOMAXPROCS=8

docker logs lab-go-counter-mutex | head -1
mech=mutex workers=32 GOMAXPROCS=8
```

Next, we can compare their throughput:
```sh
echo "mutex:";   docker logs lab-go-counter-mutex   | tail -3
mutex:
ops/sec=9885666 goroutines=33
ops/sec=10786877 goroutines=33
ops/sec=10707120 goroutines=33
...
echo "channel:"; docker logs lab-go-counter-channel | tail -3
channel:
ops/sec=4936005 goroutines=34
ops/sec=4926082 goroutines=34
ops/sec=4810901 goroutines=34
```
We can see that the mutex version is more than double the throughput of the channel version.

We can also compare CPU usage over the same window:
```sh
docker stats --no-stream lab-go-counter-mutex lab-go-counter-channel

CONTAINER ID   NAME                     CPU %     MEM USAGE / LIMIT     MEM %     NET I/O         BLOCK I/O     PIDS
c587893aa54d   lab-go-counter-mutex     222.63%   3.52MiB / 11.74GiB    0.03%     1.23kB / 126B   2.7MB / 0B    11
1a31c4b23f7e   lab-go-counter-channel   103.65%   2.246MiB / 11.74GiB   0.02%     1.27kB / 126B   1.19MB / 0B   12
```

We can also test across a range of `WORKERS` values, to see whether the mutex/channel gap holds steady, widens, or narrows as contention increases.
```sh
for W in 1 2 4 8 16 32 64; do
  for M in mutex channel; do
    docker stop lab-go-counter-${M} >/dev/null 2>&1
    docker rm lab-go-counter-${M} >/dev/null 2>&1
    docker run -d --name lab-go-counter-${M} \
      -e MECH=${M} -e WORKERS=$W \
      0105-go-locking-vs-channels-counter-mutex:latest >/dev/null
    sleep 4
    OPS=$(docker logs lab-go-counter-${M} | tail -1)
    echo "WORKERS=$W MECH=${M}: $OPS"
  done
done
```
The output is:
```sh
done
WORKERS=1 MECH=mutex: ops/sec=57463042 goroutines=2
WORKERS=1 MECH=channel: ops/sec=7642271 goroutines=3
WORKERS=2 MECH=mutex: ops/sec=27240888 goroutines=3
WORKERS=2 MECH=channel: ops/sec=7247185 goroutines=4
WORKERS=4 MECH=mutex: ops/sec=15668416 goroutines=5
WORKERS=4 MECH=channel: ops/sec=6550330 goroutines=6
WORKERS=8 MECH=mutex: ops/sec=14506363 goroutines=9
WORKERS=8 MECH=channel: ops/sec=5992791 goroutines=10
WORKERS=16 MECH=mutex: ops/sec=13253878 goroutines=17
WORKERS=16 MECH=channel: ops/sec=5723343 goroutines=18
WORKERS=32 MECH=mutex: ops/sec=11603717 goroutines=33
WORKERS=32 MECH=channel: ops/sec=5730501 goroutines=34
WORKERS=64 MECH=mutex: ops/sec=9496627 goroutines=65
WORKERS=64 MECH=channel: ops/sec=4505216 goroutines=66
```

Same data, as a table, with the mutex/channel ratio at each point:

| WORKERS | mutex ops/sec | channel ops/sec | ratio |
|---|---|---|---|
| 1 | 57,463,042 | 7,642,271 | 7.52x |
| 2 | 27,240,888 | 7,247,185 | 3.76x |
| 4 | 15,668,416 | 6,550,330 | 2.39x |
| 8 | 14,506,363 | 5,992,791 | 2.42x |
| 16 | 13,253,878 | 5,723,343 | 2.32x |
| 32 | 11,603,717 | 5,730,501 | 2.02x |
| 64 | 9,496,627 | 4,505,216 | 2.11x |


Mutex wins throughout - but the *shape* of the gap is the more interesting part. It's largest exactly where there's no real contention at all (`WORKERS=1`: 7.52x), then collapses hard by `WORKERS=4` and settles into a roughly flat ~2-2.4x floor from there on, even out to `WORKERS=64`. That split makes sense as two different regimes: at `WORKERS=1`, mutex's uncontended fast path is nearly free (a single CAS, nobody to contend with), while a channel op is *always* a two-party rendezvous - even with one sender and one owner, that's still a real hand-off between two goroutines every single time. Past `WORKERS=4`, both mechanisms are dealing with genuine contention, but at *different points*: mutex's counter cache line gets hit directly by every contending core, while the channel's counter is only ever touched by the single owner goroutine - channel's contention instead piles up at the unbuffered channel rendezvous itself (more senders competing to be the one the owner receives from next).

