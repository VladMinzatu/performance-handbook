## Burst latency scales with CONNS and shrinks with GOMAXPROCS

We test Prediction 4 here: the netpoller removes the *thread* bottleneck, but not the *CPU-parallelism* bottleneck. A burst makes every held connection's goroutine runnable at once, but only `GOMAXPROCS` of them can actually run Go code at any given instant. So the `/burst` endpoint's latency (`p50`/`p99`/`max`/`total`) should:
- grow with `CONNS` at a fixed `GOMAXPROCS`, and
- shrink when `GOMAXPROCS` is raised at a fixed `CONNS`.

Both server and client need to be recreated with different environment variables per run, so we'll only use use one-off `docker run` rather than `compose.yml` here - including for the server. The one thing `compose.yml` gives the server for free that a bare `docker run` doesn't is the `server` DNS name the client dials by default. Recreating it by hand needs `--network-alias server` to keep
that working without also having to override `SERVER_ADDR` on the client.

So first stop whatever the normal `compose.yml` setup left running:
```sh
docker stop lab-go-netpoll-client lab-go-netpoll-server
docker rm lab-go-netpoll-client lab-go-netpoll-server
```

**Sweep 1 - fixed `CONNS=5000`, `GOMAXPROCS` in {1, 2, 4, 8}:**
```sh
for G in 1 2 4 8; do
  echo "=== GOMAXPROCS=$G ==="
  docker run -d --name lab-go-netpoll-server --network labnet --network-alias server \
    -e GOMAXPROCS=$G 0102-go-netpoller-epoll-server:latest
  sleep 1
  docker run -d --name lab-go-netpoll-client --network labnet -p 8090:8090 \
    -e SERVER_ADDR=server:9000 -e CONNS=5000 0102-go-netpoller-epoll-client:latest
  sleep 5
  docker logs lab-go-netpoll-server | tail -1   # confirm active_conns=5000 before bursting
  curl -s -X POST http://localhost:8090/burst
  echo
  docker stop lab-go-netpoll-client lab-go-netpoll-server
  docker rm lab-go-netpoll-client lab-go-netpoll-server
  sleep 1
done
```
```
=== GOMAXPROCS=1 ===
goroutines=5002 active_conns=5000
{"conns":"5000","max":"526.344126ms","p50":"279.478944ms","p99":"521.151652ms","total":"551.043907ms"}

=== GOMAXPROCS=2 ===
goroutines=5002 active_conns=5000
{"conns":"5000","max":"291.046519ms","p50":"160.413511ms","p99":"288.272595ms","total":"317.927307ms"}

=== GOMAXPROCS=4 ===
goroutines=5002 active_conns=5000
{"conns":"5000","max":"150.767234ms","p50":"73.507965ms","p99":"147.567349ms","total":"168.721203ms"}

=== GOMAXPROCS=8 ===
goroutines=5002 active_conns=5000
{"conns":"5000","max":"128.977253ms","p50":"69.744246ms","p99":"126.088328ms","total":"155.283081ms"}
```

**Sweep 2 - fixed `GOMAXPROCS=2`, `CONNS` in {1000, 5000, 10000}:**
```sh
docker run -d --name lab-go-netpoll-server --network labnet --network-alias server \
  -e GOMAXPROCS=2 0102-go-netpoller-epoll-server:latest
sleep 1
for C in 1000 5000 10000; do
  echo "=== CONNS=$C ==="
  docker run -d --name lab-go-netpoll-client --network labnet -p 8090:8090 \
    -e SERVER_ADDR=server:9000 -e CONNS=$C 0102-go-netpoller-epoll-client:latest
  sleep 6
  docker logs lab-go-netpoll-server | tail -1
  curl -s -X POST http://localhost:8090/burst
  echo
  docker stop lab-go-netpoll-client
  docker rm lab-go-netpoll-client
  sleep 1
done
docker stop lab-go-netpoll-server && docker rm lab-go-netpoll-server
```
```
=== CONNS=1000 ===
goroutines=1002 active_conns=1000
{"conns":"1000","max":"81.348863ms","p50":"51.937194ms","p99":"80.754612ms","total":"87.46784ms"}

=== CONNS=5000 ===
goroutines=5002 active_conns=5000
{"conns":"5000","max":"286.804423ms","p50":"160.377137ms","p99":"284.287958ms","total":"313.646544ms"}

=== CONNS=10000 ===
goroutines=10002 active_conns=10000
{"conns":"10000","max":"534.804067ms","p50":"273.815011ms","p99":"529.365344ms","total":"571.644967ms"}
```

Both halves of our hypothesis hold cleanly in the same run's data.

**GOMAXPROCS sweep (`CONNS=5000` fixed):**

| GOMAXPROCS | p50 | p99 | max | total |
|---|---|---|---|---|
| 1 | 279ms | 521ms | 526ms | 551ms |
| 2 | 160ms | 288ms | 291ms | 318ms |
| 4 | 74ms | 148ms | 151ms | 169ms |
| 8 | 70ms | 126ms | 129ms | 155ms |

Roughly halves going 1→2→4 (551 → 318 → 169ms total - close to the 2x work-sharing you'd expect from doubling parallelism), then flattens out 4→8 (169 → 155ms) - diminishing returns once `GOMAXPROCS` approaches the host's 8 real CPUs, where other fixed costs (scheduling, the network stack, GC, general OrbStack-VM overhead) start to dominate over the per-connection hashing work itself.

**CONNS sweep (`GOMAXPROCS=2` fixed):**

| CONNS | p50 | p99 | max | total |
|---|---|---|---|---|
| 1,000 | 52ms | 81ms | 81ms | 87ms |
| 5,000 | 160ms | 284ms | 287ms | 314ms |
| 10,000 | 274ms | 529ms | 535ms | 572ms |

Total time roughly tracks connection count directly (87 → 314 → 572ms for 1,000 → 5,000 → 10,000 - close to linear), exactly what you'd expect from a fixed amount of work per connection divided across a fixed number of Ps: 10x the connections, roughly 6.5x the total time (sub-linear only because the fixed per-burst overhead - goroutine scheduling, dialing setup already amortized before the timer starts - matters less at scale).

### Extra insight: watching the run queues themselves fill and drain

Everything above infers queuing from the outside (latency numbers). The Go runtime can report its own scheduler state directly: `GODEBUG=schedtrace=<ms>` makes it print one summary line every `<ms>` milliseconds - `gomaxprocs`, how many `P`s are idle, and (appended at the end, undocumented but stable across Go versions) the global run queue length plus each `P`'s local run queue length in brackets. (`scheddetail=1` adds a full per-goroutine dump too, but at 5000 held connections that produces over 800,000 log lines in about 15 seconds.

We'll restart the server with tracing on, fixed at `GOMAXPROCS=2` and a fine 20ms sample interval so a ~300ms burst gets several samples instead of aliasing past it:
```sh
docker stop lab-go-netpoll-client lab-go-netpoll-server
docker rm lab-go-netpoll-client lab-go-netpoll-server

docker run -d --name lab-go-netpoll-server --network labnet --network-alias server \
  -e GOMAXPROCS=2 -e "GODEBUG=schedtrace=20" \
  0102-go-netpoller-epoll-server:latest
sleep 1
docker run -d --name lab-go-netpoll-client --network labnet -p 8090:8090 \
  -e SERVER_ADDR=server:9000 -e CONNS=5000 0102-go-netpoller-epoll-client:latest
sleep 6
docker logs lab-go-netpoll-server | tail -5
```
```
SCHED 7162ms: gomaxprocs=2 idleprocs=2 threads=6 spinningthreads=0 needspinning=0 idlethreads=2 runqueue=0 [0 0]
SCHED 7186ms: gomaxprocs=2 idleprocs=2 threads=6 spinningthreads=0 needspinning=0 idlethreads=2 runqueue=0 [0 0]
SCHED 7208ms: gomaxprocs=2 idleprocs=2 threads=6 spinningthreads=0 needspinning=0 idlethreads=2 runqueue=0 [0 0]
```
Idle, as expected: both `P`s idle, global queue empty, both local queues (`[0 0]`, one number per `P`) empty: 5000 parked goroutines don't touch any run queue at all, they're not runnable.

Now capture the log lines from right before a burst to just after it finishes:
```sh
LINES_BEFORE=$(docker logs lab-go-netpoll-server 2>&1 | wc -l)
curl -s -X POST http://localhost:8090/burst
sleep 0.5
docker logs lab-go-netpoll-server 2>&1 | tail -n +$((LINES_BEFORE+1))
```
```
{"conns":"5000","max":"269.618836ms","p50":"143.078675ms","p99":"267.074779ms","total":"287.426316ms"}

SCHED 23290ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=0 [103 114]
SCHED 23314ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=158 [7 38]
SCHED 23334ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=128 [3 123]
SCHED 23359ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=128 [88 90]
SCHED 23382ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=128 [123 125]
SCHED 23404ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=157 [50 21]
SCHED 23425ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=128 [114 114]
SCHED 23448ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=142 [30 12]
SCHED 23473ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=189 [6 61]
SCHED 23494ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=128 [122 119]
SCHED 23518ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=156 [23 48]
SCHED 23541ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=128 [89 87]
SCHED 23563ms: gomaxprocs=2 idleprocs=0 threads=6 spinningthreads=0 needspinning=1 idlethreads=1 runqueue=136 [2 0]
SCHED 23585ms: gomaxprocs=2 idleprocs=2 threads=6 spinningthreads=0 needspinning=0 idlethreads=2 runqueue=0 [0 0]
SCHED 23608ms: gomaxprocs=2 idleprocs=2 threads=6 spinningthreads=0 needspinning=0 idlethreads=2 runqueue=0 [0 0]
```

This is the mechanism underneath every latency number in this experiment, seen directly instead of inferred:

- **The instant the burst lands** (23290ms), both `P`s flip from idle to busy in the same 20ms sample - `idleprocs` goes 2 → 0. The global `runqueue` is still 0 at that exact instant, but each `P`'s *local* queue is already loaded (`[103 114]`) - `netpoll`'s `injectglist` (the function that turns "these fds are readable" into "these goroutines are runnable") hands most of a batch straight to `P` local queues first, only spilling into the global queue once a local queue fills up. That spillover shows up one tick later, once `runqueue` jumps to 158.
- **For the next ~270ms, both queues stay saturated and visibly churn.** Local queue values bounce around (`[3 123]` → `[88 90]`, `[6 61]` → `[122 119]`) rather than draining smoothly - consistent with Go's scheduler work-stealing: whenever one `P` empties out faster than the other, it's expected to steal from its neighbor rather than sit idle, which is exactly the kind of imbalance-then-correction pattern in these numbers. `needspinning=1` throughout confirms the runtime has correctly identified there's more runnable work than idle `P`s to give it to.
- **The queues hit zero at 23585ms** - 295ms after the burst first registered, and within one sample tick of the client's own independently measured `total: 287.43ms`. Two completely different measurement methods - the client timing round trips from outside, the Go runtime reporting its own internal queue state from inside - agree on how long the backlog took to clear, which is a real cross-check, not just two numbers that happen to look similar.

In short: Prediction 4's latency numbers aren't just correlated with `GOMAXPROCS`/`CONNS`, they're the direct, measurable consequence of a concrete, inspectable mechanism: thousands of goroutines landing on two `P` run queues at once and being worked off strictly serially per `P`, with occasional rebalancing between the two, until the backlog reaches zero.
