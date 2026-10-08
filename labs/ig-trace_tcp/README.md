# `ig trace_tcp`

Inspektor Gadget's `trace_tcp` reports the lifecycle of every TCP
connection in a container: each `connect`, `accept` and `close`, with the
process and both endpoints. Each example under [examples/](./examples)
is a small program that handles connections badly, run once with a bad
setting and once with a good one, with `trace_tcp` used to detect the bad
case. Uses the shared lab infrastructure in [tools/](../tools/README.md)
(`lab-analysis` runs `ig`).

## What it sees

One event per connection state change, captured in the kernel's TCP code.
Every connection is caught, however short-lived. Event types:

- `connect` - an outgoing connection (client side). A refused connect
  shows `error=ECONNREFUSED`.
- `accept` - an incoming connection handed to the application (server
  side).
- `close` - a connection closed, on whichever side it happens.

One HTTP request on a fresh connection, traced in both containers:
```
RUNTIME.CONTAINERNAME   COMM     SRC                   DST                   TYPE
tcp-probe-cli           python   192.168.117.4:56468   192.168.117.3:8000    connect
tcp-probe-srv           python   192.168.117.3:8000    192.168.117.4:56468   accept
tcp-probe-srv           python   192.168.117.3:8000    192.168.117.4:56468   close
tcp-probe-cli           python   192.168.117.4:56468   192.168.117.3:8000    close
```
Fields worth knowing:

| Field | What it tells you |
|---|---|
| `runtime.containerName` | Which container the event happened in |
| `proc.comm`, `proc.pid`, `proc.tid` | The process (and thread) doing the connect/accept/close |
| `type` | `connect`, `accept` or `close` |
| `src`, `dst` | Local and remote `addr:port`, from this container's point of view; a new `src` port on every `connect` means a new connection every time |
| `error` | Why a connect failed (`ECONNREFUSED`, ...); `--failure-only` shows only these |
| `fd`, `accept_fd` | The socket's file descriptor in the process |

What it doesn't see:
- Requests on a connection that's already open. A reused connection
  produces no events at all, so the absence of events is the healthy
  signal.
- Bytes, throughput or retransmits. Use `top_tcp` and `trace_tcpretrans`
  for those.
- UDP.
- Timeouts as failures. A connect that times out (no SYN-ACK) shows up as
  a plain `connect` with no error, so `--failure-only` won't list it.
  Only explicit failures such as a refused connection carry an `error`.

## Why it matters in production

Connection handling bugs rarely produce errors. Requests succeed, error
rates stay at zero, and latency gets somewhat worse. The cost sits in
things nobody is watching: a TCP handshake (and often a TLS one) per
request, CPU spent on connection setup, sockets piling up in TIME_WAIT
until ephemeral ports run out ("cannot assign requested address"), and
servers spending their accept budget on churn. Server-side metrics show
connection counts but not which client is responsible or why. Typical
sources:
- HTTP clients that don't reuse connections: response bodies not read to
  the end or not closed, a new client per request, keep-alive disabled,
  or idle pools too small for the concurrency
- connection pools that close and reopen under load: timeouts that
  discard pooled connections, short idle timeouts, a max lifetime that's
  too low
- reconnect loops against an unavailable dependency (`ECONNREFUSED` in a
  tight loop)
- unexpected destinations: a service talking to something it shouldn't,
  or skipping a proxy or sidecar it should go through

`trace_tcp` answers the questions those leave open: which process opens
connections, to where, how many per second, and which side closes them.
The side that closes first holds the TIME_WAIT entry.

## Usage

```sh
# stream every connect/accept/close in one container
docker exec lab-analysis ig run trace_tcp:latest -c <container>
# only outgoing connections, or only failed ones
docker exec lab-analysis ig run trace_tcp:latest -c <container> --connect-only
docker exec lab-analysis ig run trace_tcp:latest -c <container> --failure-only
# connection rate: connects over 10s, grouped by process and destination
docker exec lab-analysis sh -c "ig run trace_tcp:latest -c <container> --connect-only -t 10 -o json \
  | jq -r '[.proc.comm, .dst.addr + \":\" + (.dst.port|tostring)] | @tsv' | sort | uniq -c"
# both ends of a connection: event counts per container and type
docker exec lab-analysis sh -c "ig run trace_tcp:latest -c <client>,<server> -t 10 -o json \
  | jq -r '[.runtime.containerName, .type] | @tsv' | sort | uniq -c"
```
Without `-c`, it traces every container on the host. Add `--host` to
include host processes. Compare the connect rate with the request rate:
close to 1 connect per request means no connection reuse, and close to 0
means connections are reused.

## Examples

Coming next:
1. Go HTTP client response bodies not drained/closed: a new connection
   per request instead of a reused one.
