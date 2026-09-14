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

Next, compare CPU cost over the same window. This matters independently of throughput: if one mechanism reaches the same items/sec while burning noticeably more CPU, the cost is real but hidden by there being spare cores to absorb it:
```sh
docker stats --no-stream lab-go-workerpool-channel lab-go-workerpool-cond
CONTAINER ID   NAME                        CPU %     MEM USAGE / LIMIT     MEM %     NET I/O         BLOCK I/O    PIDS
ba088aeb91cf   lab-go-workerpool-channel   279.20%   3.379MiB / 11.74GiB   0.03%     1.84kB / 126B   6MB / 0B     10
ad9910eee608   lab-go-workerpool-cond      276.88%   2.469MiB / 11.74GiB   0.02%     1.58kB / 126B   922kB / 0B   12
```

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


Then sweep the worker count at a fixed, shallow queue depth, so blocking stays frequent while the number of goroutines competing to be woken grows. This is the axis that would expose a per-waiter scaling cost if one exists - the careful two-condvar design is specifically supposed *not* to have one, since `Signal` wakes a single waiter regardless of how many are queued:
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

Clean up when done:
```sh
docker compose -f compose.yml down
```
