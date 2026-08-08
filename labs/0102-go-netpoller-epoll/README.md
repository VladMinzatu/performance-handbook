# 0102 - Go's netpoller: collapsing goroutines onto epoll

Uses the shared lab infrastructure in [tools/](../tools/README.md). The
`analysis` container is only needed for the syscall-tracing predictions
(2 and 3) - the goroutine/connection counts and the burst-latency numbers
come straight from the server and client's own logs and HTTP endpoint.

## Background

Go's pitch for network servers is "spawn a goroutine per connection, block
freely." That's only cheap because blocking on network I/O in Go isn't
really blocking an OS thread at all. When a goroutine calls `Read`/`Write`
on a `net.Conn` (or `Dial`/`Accept`) and the socket isn't ready, the
runtime puts the file descriptor in non-blocking mode, registers it with a
kernel readiness API - `epoll` on Linux, via `epoll_ctl(EPOLL_CTL_ADD, fd,
...)` - and parks the goroutine (`gopark`), freeing the OS thread (M) it
was running on to go do other work. A small number of the runtime's own
threads take turns calling `epoll_wait` (from the scheduler's
`findrunnable`, or from `sysmon`) to collect whichever of the - potentially
thousands - registered fds have become ready, and hand the corresponding
parked goroutines back to a run queue (`netpollready`).

The practical upshot: thousands of goroutines blocked on network reads
cost a parked goroutine stack and one epoll registration each, not an OS
thread each. This is specifically a network-I/O story - it relies on the
netpoller's special integration with the scheduler. It does *not* extend
to every kind of "blocking" call in Go (a genuinely blocking syscall, e.g.
file I/O or cgo, really does tie up an M and can force the runtime to spin
up another one) - that contrast is its own separate lab; this one stays
focused on what the netpoller does for sockets specifically.

## Hypotheses

**Prediction 1 - goroutine count scales with connections; OS thread count
doesn't.** Holding `CONNS` concurrent connections open (each with a
server-side goroutine parked in `Read`) should make the server's own
`runtime.NumGoroutine()` track `CONNS` almost 1:1, while the number of OS
threads backing the server process (`/proc/<pid>/status`'s `Threads:`
field, or `ls /proc/<pid>/task | wc -l`) stays roughly flat at
`GOMAXPROCS` plus a small constant (sysmon, GC workers, the netpoller
thread itself) - whether `CONNS` is 100 or 10,000.

**Prediction 2 - the mechanism is one shared epoll instance, not one wait
per goroutine.** Tracing syscalls on the server's PID while connections
ramp up should show `epoll_ctl` (op `EPOLL_CTL_ADD`) called roughly once
per accepted connection - tracking `CONNS` - but `epoll_wait` calls should
*not* scale with connection count. A handful of threads, bounded by
`GOMAXPROCS`, repeatedly wait on the same epoll fd for readiness across
every registered socket at once, each call potentially returning many
ready fds in a single batch.

**Prediction 3 - waking a parked goroutine doesn't create a thread.**
Triggering the client's `/burst` endpoint - which writes one byte to
every held connection at (as close to) the same instant as possible -
should make thousands of server-side goroutines runnable in a very short
window. If the netpoller is doing what it claims, none of that should
show up as new OS thread creation: tracing `clone`/`sched_process_fork`
on the server's PID during a burst should show zero or near-zero new
threads, in contrast to what a thread-per-blocking-read design would
require to service the same event.

**Prediction 4 (stretch) - the netpoller removes the thread bottleneck,
not the CPU bottleneck.** A burst makes `CONNS` goroutines runnable at
once, but only `GOMAXPROCS` of them can actually execute Go code at any
given instant - the rest sit on run queues waiting for a P. So the
`/burst` endpoint's reported latency distribution (`p50`/`p99`/`max`,
returned as JSON) should show a visible tail that grows with `CONNS` at a
fixed `GOMAXPROCS`, and should shrink when `GOMAXPROCS` is raised for the
same `CONNS` - the same GOMAXPROCS-vs-parallelism story as
[lab 0101](../0101-go-gomaxprocs-cgroups/README.md), now triggered by a
netpoller wakeup storm instead of a busy-loop.

## Setup

Build and start the server and client, joined to `labnet`:
```sh
docker compose -f compose.yml up -d --build
```
The server listens on `:9000` with `GOMAXPROCS=2` (deliberately
constrained - see Prediction 4); the client dials `CONNS=2000` connections
to it in small batches and holds them open, exposing a burst controller on
`:8090`.

Watch each side's own view of the connection/goroutine count:
```sh
docker logs -f lab-go-netpoll-server   # goroutines=N active_conns=N, once/sec
docker logs -f lab-go-netpoll-client   # held_conns=N, once/sec while dialing and after
```

Check the server's real OS thread count directly (the server binary runs
as PID 1 in its container):
```sh
docker exec lab-go-netpoll-server sh -c 'grep Threads /proc/1/status'
docker exec lab-go-netpoll-server sh -c 'ls /proc/1/task | wc -l'
```

Trigger a synchronized wake-all burst and see the round-trip latency
distribution it produces:
```sh
curl -X POST http://localhost:8090/burst
```
Returns JSON: `conns` (how many connections were bursted),
`total` (wall time for the whole burst to finish), and `p50`/`p99`/`max`
per-connection round-trip latency.

To compare against a different `GOMAXPROCS` or `CONNS`, edit the
`environment:` blocks in `compose.yml` and rebuild:
```sh
docker compose -f compose.yml up -d --build
```

For Predictions 2 and 3, trace the server's syscalls from the shared
analysis container (see [tools/](../tools/README.md) for the one-time
`labnet`/`analysis` setup):
```sh
docker compose -f ../tools/analysis/compose.yml exec analysis bash
# inside the analysis container:
pgrep netpoll-server
bpftrace -e 'tracepoint:syscalls:sys_enter_epoll_ctl /pid == <PID>/ { @op[args->op] = count(); }'   # 1=ADD 2=DEL 3=MOD
bpftrace -e 'tracepoint:syscalls:sys_enter_epoll_wait /pid == <PID>/ { @ = count(); }'
bpftrace -e 'tracepoint:syscalls:sys_enter_clone /pid == <PID>/ { @ = count(); }'
```

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down
```
