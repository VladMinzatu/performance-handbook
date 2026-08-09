## Goroutines vs. OS threads, as connection count scales

We test Prediction 1: `runtime.NumGoroutine()` should track `CONNS` almost 1:1, while the server's real OS thread count should stay flat at roughly `GOMAXPROCS` plus a small constant, regardless of `CONNS`.

Having run the lab's normal setup (server with `GOMAXPROCS=2`, client
holding the default `CONNS=2000`):
```sh
docker compose -f ../compose.yml up -d --build
```

To sweep other `CONNS` values without editing `compose.yml`, stop the compose-managed client and launch one-off client containers on the same network instead, pointed at the same server:
```sh
docker stop lab-go-netpoll-client && docker rm lab-go-netpoll-client

docker run --rm -d --name lab-go-netpoll-client --network labnet \
  -e SERVER_ADDR=server:9000 -e CONNS=100 \
  0102-go-netpoller-epoll-client:latest

..

docker logs lab-go-netpoll-server | tail -3
goroutines=102 active_conns=100
goroutines=102 active_conns=100
goroutines=102 active_conns=100

docker exec lab-go-netpoll-server sh -c 'grep Threads /proc/1/status'
Threads:	6

docker exec lab-go-netpoll-server sh -c 'ls /proc/1/task | wc -l'
6
```

Repeating this for 2000 connections:
```sh
docker stop lab-go-netpoll-client

docker run --rm -d --name lab-go-netpoll-client --network labnet \
  -e SERVER_ADDR=server:9000 -e CONNS=2000 \
  0102-go-netpoller-epoll-client:latest

docker logs lab-go-netpoll-server | tail -3
goroutines=2002 active_conns=2000
goroutines=2002 active_conns=2000
goroutines=2002 active_conns=2000

docker exec lab-go-netpoll-server sh -c 'grep Threads /proc/1/status'
Threads:	6

docker exec lab-go-netpoll-server sh -c 'ls /proc/1/task | wc -l'
6
```

Repeating this for 10_000 connections:
```sh
docker stop lab-go-netpoll-client

docker run --rm -d --name lab-go-netpoll-client --network labnet \
  -e SERVER_ADDR=server:9000 -e CONNS=10000 \
  0102-go-netpoller-epoll-client:latest

docker logs lab-go-netpoll-server | tail -3
goroutines=10002 active_conns=10000
goroutines=10002 active_conns=10000
goroutines=10002 active_conns=10000

docker exec lab-go-netpoll-server sh -c 'grep Threads /proc/1/status'
Threads:	6

docker exec lab-go-netpoll-server sh -c 'ls /proc/1/task | wc -l'
6
```

`goroutines` is always `active_conns + 2` (one goroutine per held connection, plus `main` and the once-a-second stats ticker), so it tracks `CONNS` exactly, as expected. `Threads` (`/proc/1/status`, corroborated by `ls /proc/1/task | wc -l`) is **6 in all three runs** - unchanged from 100 connections all the way to 10_000.

This confirms Prediction 1 outright: goroutine count scales 1:1 with held connections, OS thread count doesn't move at all across a 100x range of connection count. The 5 threads are the constant the runtime needs regardless of load - not measured individually here, but consistent with 2 (`GOMAXPROCS`) scheduler threads plus a small fixed number of runtime-internal threads (`sysmon`, template thread, etc.), not one-per-network-goroutine. If Go's netpoller weren't collapsing these blocked reads onto a shared epoll wait, a thread-per-connection design would show `Threads` scaling right alongside `active_conns` - 10,000 connections would mean on the order of 10,000 threads, not 6.

Note this only says thread *count* is flat - it says nothing about the
cost of actually *running* code once all those parked goroutines wake up
at once.
