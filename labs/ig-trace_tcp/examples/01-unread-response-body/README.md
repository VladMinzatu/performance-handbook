# Unread response bodies: a new connection per request

A service that checks whether orders exist by calling an orders API. It
only needs the status code, so it never reads the ~1KB JSON body. That's
the same shape as health checkers, webhook senders and "does X exist"
lookups. It uses one shared `http.Client` with a 5s timeout, the way most
Go services do. The one setting is `BODY`, what happens to the unread
body:

- `BODY=leak` (bad, default): nothing. The body is never read and never
  closed.
- `BODY=close` (bad): `resp.Body.Close()` without reading it.
- `BODY=drain` (good): `io.Copy(io.Discard, resp.Body)`, then `Close()`.

Go's transport returns a connection to its idle pool only when the
response body has been read to EOF and then closed. A body that's never
closed keeps its connection, and the read/write goroutines that serve it,
tied up until something tears it down. Here that's the client's 5s
timeout. A body closed with unread bytes still on the wire can't be
reused either, so the transport closes the connection. In both bad cases
every request needs a new TCP connection. Requests still succeed, error
rates stay at zero, and the cost shows up somewhere else: a handshake per
request, sockets in TIME_WAIT, and in the `leak` case, goroutines and file
descriptors held for the length of the timeout.

The code that *does* read the body has a subtler version of the bug.
When the server sends a `Content-Length` and the reader consumes the last
byte (`json.Decoder` on a small response, for example), Go hands back
EOF along with that byte. The connection is reused even if `Close()` is
never called. Larger, chunked responses lose that, and reuse then depends
on whether the terminating chunk happened to be buffered: an intermittent
leak. Code that never reads the body, as here, leaks on every request.

## Setup

Start the analysis container if it isn't running, then this example,
from this directory:
```sh
docker compose -f ../../../tools/analysis/compose.yml up -d
docker compose up -d --build                  # BODY=leak
BODY=close docker compose up -d               # switch setting (recreates the client)
```
- `tcp01-server` - the orders API on `:8080`: `GET /orders/{id}` returns
  the order as JSON.
- `tcp01-client` - the order checker: `RATE` req/s (default `200`), open
  loop, against `http://tcp01-server:8080`. Logs every 5s: successful
  requests, errors, p50/p99 latency, goroutines and open fds.

Both are on `labnet`. Observation commands used below:
```sh
# the client's own view
docker logs --tail 4 tcp01-client
# connects over 10s, by process and destination
docker exec lab-analysis sh -c "ig run trace_tcp:latest -c tcp01-client --connect-only -t 10 -o json \
  | jq -r '[.proc.comm, .dst.addr + \":\" + (.dst.port|tostring)] | @tsv' | sort | uniq -c"
# the client's sockets by state
PID=$(docker inspect -f '{{.State.Pid}}' tcp01-client)
for st in established time-wait; do printf '%s ' $st
  docker exec lab-analysis nsenter --net=/proc/$PID/ns/net ss -tanH state $st | wc -l; done
```

## Bad setting: `BODY=leak`

```sh
docker compose up -d --build
docker logs --tail 4 tcp01-client
```
producing output:
```
t=  15s ok= 1000 err=  0 p50=1.49ms   p99=5.79ms   goroutines=2004   fds=1006
t=  20s ok= 1000 err=  0 p50=1.78ms   p99=4.4ms    goroutines=2007   fds=1007
t=  25s ok= 1000 err=  0 p50=1.79ms   p99=4.39ms   goroutines=2004   fds=1007
t=  30s ok= 1000 err=  0 p50=1.62ms   p99=5.38ms   goroutines=2007   fds=1008
```
Every request succeeds in under 2ms at the median. The only clues are
the client holding ~2000 goroutines and ~1000 open file descriptors to
make 200 req/s against a single host.

### Detecting it with `trace_tcp`

Watch the client's connection events:
```sh
docker exec lab-analysis sh -c "ig run trace_tcp:latest -c tcp01-client -t 1 -o json \
  | jq -r '[.type, .src.port, .dst.addr + \":\" + (.dst.port|tostring)] | @tsv' | head -6"
```
producing output:
```
close	53796	192.168.117.3:8080
close	53784	192.168.117.3:8080
connect	34856	192.168.117.3:8080
connect	34858	192.168.117.3:8080
close	53812	192.168.117.3:8080
close	53810	192.168.117.3:8080
```
A steady stream of new connections to the same server, each from a new
source port, and a stream of closes for *other* ports: connections opened
earlier are only now going away. Count the connects:
```sh
docker exec lab-analysis sh -c "ig run trace_tcp:latest -c tcp01-client --connect-only -t 10 -o json \
  | jq -r '[.proc.comm, .dst.addr + \":\" + (.dst.port|tostring)] | @tsv' | sort | uniq -c"
```
producing output:
```
   2000 client	192.168.117.3:8080
```
2000 connects in 10s at 200 req/s is exactly one per request: no reuse at
all. To see how long each connection lives, record 8s of both containers,
keyed by the client's port, then follow the first connection opened:
```sh
docker exec lab-analysis sh -c "ig run trace_tcp:latest -c tcp01-client,tcp01-server -t 8 -o json \
  | jq -r '[.timestamp_raw, .runtime.containerName, .type,
            (if .runtime.containerName==\"tcp01-client\" then .src.port else .dst.port end)] | @tsv'" > /tmp/tcp.txt
P=$(awk -F'\t' '$3=="connect"{print $4; exit}' /tmp/tcp.txt)
awk -F'\t' -v p="$P" '$4==p' /tmp/tcp.txt | awk -F'\t' 'NR==1{t0=$1} {printf "+%.3fs\t%s\t%s\t%s\n", ($1-t0)/1e9, $2, $3, $4}'
```
producing output:
```
+0.000s	tcp01-client	connect	44404
+0.000s	tcp01-server	accept	44404
+5.001s	tcp01-client	close	44404
+5.001s	tcp01-server	close	44404
```
Across all the connections that opened and closed within the window:
```sh
awk -F'\t' '$2=="tcp01-client" && $3=="connect"{c[$4]=$1}
  $2=="tcp01-client" && $3=="close" && ($4 in c){printf "%.2f\n", ($1-c[$4])/1e9}' /tmp/tcp.txt \
  | sort -n | awk '{a[NR]=$1} END{print "n="NR, "min="a[1]"s", "median="a[int(NR/2)]"s", "max="a[NR]"s"}'
```
producing output:
```
n=600 min=5.00s median=5.00s max=5.01s
```
Connect-to-close was 5.00-5.01s every time, the client's
`http.Client.Timeout`. The request completed in a couple of milliseconds,
and then the connection sat idle, unusable, until the timeout fired. So
a connection lifetime pinned at a timeout value, with one connect per
request, is the signature of a body that's never closed. With no timeout
these connections would never be released, and the goroutines and fds
would grow until the process hit its fd limit.

The client's sockets agree:
```
established     1000
time-wait    10095
```
That's 1000 ESTABLISHED (200 req/s × 5s), and TIME_WAIT on the client
because the client closes first.

## Bad setting: `BODY=close`

```sh
BODY=close docker compose up -d
docker logs --tail 4 tcp01-client
```
producing output:
```
t=  15s ok= 1000 err=  0 p50=1.86ms   p99=5ms      goroutines=4      fds=6
t=  20s ok= 1000 err=  0 p50=1.89ms   p99=6.6ms    goroutines=5      fds=6
t=  25s ok= 1000 err=  0 p50=1.81ms   p99=4.2ms    goroutines=9      fds=8
t=  30s ok= 1000 err=  0 p50=1.51ms   p99=5.09ms   goroutines=5      fds=6
```
Closing the body fixes the goroutine and fd growth. From inside the
process this now looks healthy. But the connect count is unchanged:
```
   2000 client	192.168.117.3:8080
```
Still one connection per request. The same per-port trace (2s this time,
absolute timestamps) shows each connection's whole life:
```
2026-10-08T18:36:42.094284118Z	tcp01-client	connect	35660
2026-10-08T18:36:42.094309869Z	tcp01-server	accept	35660
2026-10-08T18:36:42.094449286Z	tcp01-client	close	35660
2026-10-08T18:36:42.094518745Z	tcp01-server	close	35660
```
The connection lives 165µs, about one request. The client closes first,
so the client holds the TIME_WAIT entries:
```
established        0
time-wait    11066
```
That's ~11,000 sockets in TIME_WAIT, close to 200 new connections/s ×
60s. Every one of them pins a local port to the same server, out of the
~28,000 in Linux's default ephemeral range (32768-60999). At around
470 new connections/s the range is used up, and connects start failing
with `cannot assign requested address`.
`trace_tcp` tells `close` from `leak` by connection lifetime: microseconds
here, the full timeout there.

## Good setting: `BODY=drain`

```sh
BODY=drain docker compose up -d
docker logs --tail 4 tcp01-client
```
producing output:
```
t=  15s ok= 1000 err=  0 p50=460µs    p99=1.26ms   goroutines=8      fds=8
t=  20s ok= 1000 err=  0 p50=480µs    p99=1.84ms   goroutines=8      fds=8
t=  25s ok= 1000 err=  0 p50=450µs    p99=1.11ms   goroutines=8      fds=8
t=  30s ok= 1000 err=  0 p50=490µs    p99=1.3ms    goroutines=8      fds=8
```
Median latency drops from ~1.8ms to ~0.47ms, because no request pays for
a TCP handshake any more. Goroutines and fds stay flat. Counting connects
for 30s (`-t 30` in the connect-count command) produces no output: zero
new connections in 30s at 200 req/s. Over a separate 10s window, the
event counts for both containers were 2 of each:
```sh
docker exec lab-analysis sh -c "ig run trace_tcp:latest -c tcp01-client,tcp01-server -t 10 -o json \
  | jq -r '[.runtime.containerName, .type] | @tsv' | sort | uniq -c"
```
producing output:
```
      2 tcp01-client	close
      2 tcp01-client	connect
      2 tcp01-server	accept
      2 tcp01-server	close
```
Requests are served over a couple of long-lived connections. An
occasional burst briefly needs one more, which is then closed again. Go
keeps only 2 idle connections per host by default
(`MaxIdleConnsPerHost`). The client's sockets:
```
established        2
time-wait       17
```

## Takeaway

An HTTP client reuses a connection only if every response body is read
to EOF and closed. Skipping either produces no errors, only a new
connection per request. `trace_tcp` shows it from outside the process,
without changing code: a connect rate equal to the request rate means no
reuse. The connection lifetime tells the two bugs apart. A lifetime pinned
at the client timeout means the body is never closed (goroutines and fds
grow too). A lifetime of one request means it's closed without being read
(TIME_WAIT grows toward port exhaustion). The fix in both cases is
`io.Copy(io.Discard, resp.Body)` before `resp.Body.Close()`, on every
code path, including error and status-only ones.

## Tear down

```sh
docker compose down
```
