## Thread count by blocking mechanism: timer vs. syscall vs. netpoll

We test Prediction 1: with the same `WORKERS=2000` goroutines each blocking for the same `WORK_MS=200ms` per iteration, `MODE=syscall`'s OS thread count should climb well above `GOMAXPROCS` plus a small constant, while `MODE=timer` and `MODE=netpoll` should both stay flat near
`GOMAXPROCS` plus a small constant - matching [lab 0102](../../0102-go-netpoller-epoll/README.md)'s finding for network I/O, and extending it with `time.Sleep`'s even more basic case (no
syscall happens at all).

First, build and start the companion delay-server and all three workers:
```sh
docker compose -f compose.yml up -d --build
```

Confirm each worker's startup line (mode + GOMAXPROCS):
```sh
docker logs lab-go-worker-timer | head -1
mode=timer workers=2000 work_ms=200 GOMAXPROCS=8


docker logs lab-go-worker-syscall | head -1
mode=syscall workers=2000 work_ms=200 GOMAXPROCS=8


docker logs lab-go-worker-netpoll | head -1
mode=netpoll workers=2000 work_ms=200 GOMAXPROCS=8


docker logs lab-go-delay-server | head -1
delay-server listening on :9100 delay=200ms GOMAXPROCS=8
```

Check goroutine counts - this part should look identical across all three (all `~2000`)
```sh
docker logs lab-go-worker-timer | tail -1
goroutines=2001

docker logs lab-go-worker-syscall | tail -1
goroutines=2001

docker logs lab-go-worker-netpoll | tail -1
goroutines=2001
```

Next, check real OS thread counts: here we predict that they should diverge:
```sh
echo "timer:";   docker exec lab-go-worker-timer   sh -c 'grep Threads /proc/1/status'
timer:
Threads:	11


echo "syscall:"; docker exec lab-go-worker-syscall sh -c 'grep Threads /proc/1/status'
syscall:
Threads:	2011

echo "netpoll:"; docker exec lab-go-worker-netpoll sh -c 'grep Threads /proc/1/status'
netpoll:
Threads:	17
```

Corroborate with the task count directly, in case `/proc/1/status` and
`ls /proc/1/task` ever disagree:
```sh
echo "timer:";   docker exec lab-go-worker-timer   sh -c 'ls /proc/1/task | wc -l'
timer:
11


echo "syscall:"; docker exec lab-go-worker-syscall sh -c 'ls /proc/1/task | wc -l'
syscall:
2011


echo "netpoll:"; docker exec lab-go-worker-netpoll sh -c 'ls /proc/1/task | wc -l'
netpoll:
17
```

This confirms our first hypothesis. `MODE=syscall` sits at **2011 threads** for 2000 blocked goroutines - essentially one OS thread per concurrently-blocked `nanosleep` call. `MODE=timer` (11 threads) and `MODE=netpoll` (17 threads) both stay in that same small, flat ballpark regardless of `WORKERS` - not scaling with the number of blocked goroutines at all.
