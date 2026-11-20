# Memory per idle connection: a goroutine-per-connection server pays for a stack and buffers on every idle socket

Uses the shared lab infrastructure in [tools/](../tools/README.md). The
`analysis` container reads each server's cgroup memory files and counts
its connections with `ss`, without any help from the servers' own code.

## Background

Some servers spend most of their life holding connections that do
nothing. A client connects, subscribes to something, and then waits,
sometimes for hours, for the server to push it a message:

- **WebSocket and SSE gateways**: chat, notifications, presence,
  collaborative editing, live sports scores and trading feeds. A single
  instance commonly holds 10^4-10^5 connections, and a fleet millions.
- **MQTT brokers** for IoT devices, which keep a connection open for
  days and send a few bytes every few minutes.
- **Long-lived gRPC streams** such as Kubernetes API watches, etcd
  watches and xDS control planes (Istio's `istiod` holds a stream to
  every Envoy sidecar in the mesh).
- **HTTP keep-alive connections** from large client fleets. 500 client
  pods each keeping up to 100 idle connections to a service can leave
  it holding 50,000 connections that carry no requests. Go's
  `http.Server` keeps idle keep-alives open forever unless `IdleTimeout`
  (or `ReadTimeout`, which it falls back to) is set.

For these servers, the limit isn't CPU or request rate. It's memory per
connection × connections, and that cost is set by what each connection
holds while it waits.

### What a waiting connection holds in Go

Go's standard model is one goroutine per connection, blocked in `Read`.
That's cheap compared with a thread per connection, since the netpoller
parks the goroutine and its OS thread goes off to do other work. But a
parked goroutine still holds:

- **Its stack.** At least 2KB, and more if the call chain to `Read` is
  deep or anything earlier in the goroutine's life grew it. Stacks grow
  by doubling. Since Go 1.19, new goroutines start with a stack sized to
  the average the runtime has recently seen, not always 2KB, and the GC
  shrinks a stack only when it uses less than a quarter of it.
- **The buffer passed to `Read`.** A goroutine blocked in
  `conn.Read(buf)` holds `buf` the whole time it waits, because the data
  will be copied into it when it arrives. With `bufio.NewReader` that's
  4KB, whether the next message comes in 1ms or 1 hour.
- **Whatever else the connection's code allocated and kept**, such as a
  `bufio.Writer` (another 4KB) and a channel feeding it.

Common code shapes double this. The usual WebSocket pattern, for example
in `gorilla/websocket`'s chat example, runs two goroutines per
connection: a *read pump* blocked reading from the socket and a *write
pump* blocked on a channel of outgoing messages. Each has its own stack,
and the connection keeps a 4KB read buffer and a 4KB write buffer.
`net/http`'s server likewise keeps a 4KB `bufio.Reader` and a 4KB
`bufio.Writer` for every connection, including idle keep-alives, and
HTTP/2 adds a second goroutine per connection.

None of this is garbage. It's live memory the program is still using,
so the GC can't reclaim it. Setting `GOMEMLIMIT` just makes the GC run
more and more often as the heap approaches the limit, without freeing
anything.

On top of the process's own memory, the **kernel** holds per-socket
state for every connection: the TCP socket itself, its file, and its
`epoll` registration. For an idle socket with empty send and receive
queues that's a few KB. It's the same for any runtime, and it's charged
to the container's cgroup as kernel memory.

### What it holds in tokio

A tokio task is a future: a state machine whose size the compiler fixes
from the variables that are live across each `.await`. A task waiting on
a socket holds only that state, typically a few hundred bytes, with no
stack. Tokio also lets code wait for a socket to be *readable* without
handing it a buffer (`TcpStream::readable().await`), and only allocate
or borrow one once data is there to read.

So the runtime's floor per idle connection is much lower than Go's. But
buffers are a choice in both: a tokio server built with the same two
pumps and the same 4KB buffers holds about the same buffer memory as the
Go one, and only saves the stacks.

### What to tune

- **How many goroutines a connection keeps.** One goroutine per
  connection instead of a read pump and a write pump halves the stacks.
  Writes can go straight to the socket under a mutex, or through a
  shared writer for many connections.
- **What's held while idle.** In Go, wait in `Read` with a small buffer
  (a few bytes), and take a full-size buffer from a `sync.Pool` only
  while there's data to process. `gorilla/websocket` supports the write
  side of this with `WriteBufferPool`. In tokio, wait with
  `readable().await`.
- **Not keeping a goroutine per connection at all.** At 10^5-10^6
  connections, Go servers often switch to event-loop libraries that use
  `epoll` directly, such as `gobwas/ws` with its netpoller, `gnet` or
  CloudWeGo's `netpoll`. Mail.Ru's write-up "A Million WebSockets and
  Go" describes exactly this move. The cost is giving up Go's
  straight-line blocking code for callbacks.
- **How many connections there can be.** `http.Server.IdleTimeout`
  closes idle keep-alives. A limit on accepted connections, such as
  `golang.org/x/net/netutil.LimitListener`, makes the server refuse
  connections instead of running out of memory.
- **The memory limit,** sized as base memory + per-connection cost ×
  the most connections a pod can get. That maximum is set by the worst
  case, not the average: after a deploy or a failover, clients
  reconnect to whichever pods are up, and a few pods can briefly take
  far more than their share.

### How it shows up

A server running out of memory because of idle connections doesn't
look busy. Request rate, CPU and latency stay flat. What gives it away:

- **Memory follows the connection count, not the traffic.** Memory
  (`memory.current`, mostly `anon` in `memory.stat`) rises and falls with
  the number of established connections (`ss`), in a straight line. The
  slope, Δmemory / Δconnections, is the per-connection cost.
- **In Go, goroutines follow connections too,** at a fixed ratio: about
  1 per connection, or 2 with pumps. The runtime's stack memory
  (`/memory/classes/heap/stacks:bytes` in `runtime/metrics`, or
  `go_memstats_stack_inuse_bytes` in Prometheus' Go collector) grows with
  them, and a heap profile's in-use space is dominated by
  `bufio.NewReaderSize` and `bufio.NewWriterSize`.
- **Memory doesn't come back right away.** When connections close, a Go
  server's memory stays near its peak: the runtime keeps freed memory
  for reuse and returns it to the OS only gradually. So memory reflects
  the most connections the server has held recently, not the number it
  holds now.
- **The failure is an OOM kill during a reconnect storm.** The cgroup's
  `memory.events` shows an `oom_kill` (Kubernetes reports the container
  as `OOMKilled`), and every client of that pod is disconnected at once.
  Idle clients only notice at their next heartbeat, so they reconnect
  over the heartbeat interval, to the other pods, which can push them
  over too.

## Hypotheses

A client opens 50,000 connections to a server, subscribes on each and
then leaves it idle apart from a small ping every 30s. The server
answers each ping and otherwise just waits. Each server runs one of two
modes: `pumps`, a read goroutine/task and a write goroutine/task per
connection with 4KB read and write buffers, or `lean`, one
goroutine/task per connection holding no buffer while it waits.

**Prediction 1: in Go with pumps, memory grows by about 15KB per
connection, in a straight line.** Going from 10,000 to 50,000
connections, the server's `memory.current` should grow by about 600MB,
with no requests other than the pings, and the server should run 2
goroutines per connection. The runtime should attribute about 4KB per
connection to stacks (2 goroutines at the minimum 2KB each) and about
10KB to heap objects (the two 4KB buffers, the channel and the
connection's own structures). On top of that, the kernel holds about
4KB per socket.

**Prediction 2: holding less while idle cuts that by about 4x.** With
the Go server in `lean` mode, the same connections should cost about
3.5-4KB each in the process: one 2KB stack and under 1KB of heap. 1
goroutine per connection. The kernel's 4KB per socket doesn't change,
so it's now about half of the total.

**Prediction 3: tokio only wins once the buffers are gone.** With the
same pumps and 4KB buffers, the tokio server should cost about 11KB per
connection in the process: the same buffers as Go, minus the stacks.
In `lean` mode it should drop to about 1-1.5KB, about a third of Go's
lean cost, from a task of a few hundred bytes. At that point the
kernel's socket memory is most of what a connection costs.

**Prediction 4 (stretch): the untuned server is the one that gets
OOM-killed.** With a 512MB memory limit and no swap, the Go server in
`pumps` mode should be OOM-killed at about 33,000-35,000 connections,
long before the client reaches 50,000. The client should see its
connections fail gradually, each at its next ping, over the 30s ping
interval. The same server in `lean` mode should hold all 50,000 in
about 185MB.

## Setup

Start the analysis container and the lab, from this lab's directory:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
docker compose -f compose.yml up -d --build
```
- `lab-go-idleconn-goserver` - the Go server. A TCP push-gateway
  protocol: the client sends `SUB <topic>`, the server answers `OK`, and
  after that the connection only carries `PING`/`PONG`. `MODE` picks how
  each connection is handled:
  - `pumps` (default) - a read goroutine with a 4KB `bufio.Reader` and a
    write goroutine draining a channel through a 4KB `bufio.Writer`,
    the shape of the common WebSocket read-pump/write-pump pattern.
  - `lean` - one goroutine waiting in `Read` with a 16-byte buffer. It
    takes a 4KB buffer from a `sync.Pool` only while it has data to
    process, and writes replies directly to the socket.

  Logs every 5s: connections, goroutines, and from `runtime/metrics`
  the stack memory, heap object memory (both in total and per
  connection) and the size new goroutine stacks currently start at.
- `lab-go-idleconn-tokioserver` - the same server in Rust, on tokio's
  multi-thread runtime, with the same two modes:
  - `pumps` (default) - a read task with a 4KB `BufReader` and a write
    task draining an `mpsc` channel through a 4KB `BufWriter`. The
    buffer sizes match Go's rather than tokio's default of 8KB, so the
    difference is the runtime, not the buffers.
  - `lean` - one task waiting in `readable().await`. Only once data is
    there does it read into a buffer on the worker thread's stack, which
    isn't part of the task.

  Logs the size of each kind of task once, at the first connection, and
  every 5s the number of connections.
- `lab-go-idleconn-client` - opens `CONNS` connections (default 50,000)
  to `TARGET` (`go` or `tokio`, default `go`), `RAMP` new connections
  per second (default 2,000), sends `SUB` on each and then a `PING`
  every `PING_EVERY` (default 30s) at a random offset. Its network
  namespace's ephemeral port range is widened to 1024-65535, since one
  client address can open only about 28,000 connections to one server
  port with the default range. Logs every 5s: connections open,
  connection errors and resets.

Both servers have 2 cores each, and no memory limit by default.
`MEM_LIMIT` sets one on both (for example `512m`), with swap capped at
the same value, so in effect no swap, as on a typical Kubernetes node.
With Docker's default swap allowance, a server that hits its limit is
pushed into swap instead of being OOM-killed. All three are on
`labnet`.

The kernel's per-socket memory normally shows up in the cgroup's
`memory.stat` as `kernel`, `slab` and `sock`. On OrbStack it doesn't:
its kernel boots with `cgroup.memory=nokmem,nosocket`, so kernel memory
isn't charged to containers at all, and those fields stay at 0. On this
setup `memory.current` is the process's own memory, and the kernel's
share is measured from the VM's total slab memory instead (`Slab` in
`/proc/meminfo`). That covers both ends of each connection, the
server's socket and the client's.

Observation commands used in the experiments:
```sh
# each side's own view
docker logs --tail 5 lab-go-idleconn-goserver
docker logs --tail 5 lab-go-idleconn-client

# the server's cgroup, as seen from the analysis container
SRV=$(docker inspect --format '{{.Id}}' lab-go-idleconn-goserver)
CG=/host/sys/fs/cgroup/docker/$SRV

# total memory, and how it splits between the process and the kernel
docker exec lab-analysis cat $CG/memory.current
docker exec lab-analysis grep -E '^(anon|kernel|slab|sock) ' $CG/memory.stat

# times the cgroup hit its limit, and OOM kills, while the container runs
docker exec lab-analysis grep -E '^(max|oom_kill) ' $CG/memory.events
# after an OOM kill the container and its cgroup are gone; Docker keeps the verdict
docker inspect --format 'status={{.State.Status}} oomkilled={{.State.OOMKilled}} exit={{.State.ExitCode}}' lab-go-idleconn-goserver

# the VM's slab memory, in kB: kernel socket state for both ends of every connection
docker exec lab-analysis grep '^Slab:' /proc/meminfo

# established connections on the server's port
PID=$(docker inspect --format '{{.State.Pid}}' lab-go-idleconn-goserver)
docker exec lab-analysis nsenter --net=/proc/$PID/ns/net ss -tnH state established '( sport = :9000 )' | wc -l
```
The per-connection cost is the difference in `memory.current` between
two connection counts, divided by the difference in connections. Taking
the difference subtracts the server's baseline memory, which is the same
at both counts.

To change the number of connections, or the target, recreate the
client. Its connections close, and it opens new ones at `RAMP` per
second. `--no-deps` keeps compose from recreating the servers, and
`--force-recreate` starts the client over even if its settings haven't
changed:
```sh
CONNS=10000 docker compose -f compose.yml up -d --no-deps --force-recreate client
TARGET=tokio docker compose -f compose.yml up -d --no-deps --force-recreate client
```
To change a server's mode or memory limit, recreate it. Set both each
time: a setting left out goes back to its default.
```sh
MODE=lean docker compose -f compose.yml up -d --no-deps goserver
MEM_LIMIT=512m MODE=pumps docker compose -f compose.yml up -d --no-deps goserver
```

## Experiments

See [Experiments directory](./experiments):
1. [Go with read and write pumps](./experiments/01_go_pumps.md) -
   Prediction 1.
2. [Go holding nothing while idle](./experiments/02_go_lean.md) -
   Prediction 2.
3. [The same two shapes in tokio](./experiments/03_tokio.md) -
   Prediction 3.
4. [A reconnect storm under a memory limit](./experiments/04_memory_limit.md) -
   Prediction 4.

## Tear down

```sh
docker compose -f compose.yml down
```
To stop the analysis container as well:
```sh
docker compose -f ../tools/analysis/compose.yml down
```
