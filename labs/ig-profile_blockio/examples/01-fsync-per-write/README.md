# fsync per write: a disk round trip on every request

An event-ingest service: `POST /events` appends the event as one line to
a log file and replies only once the line is on disk, so an acknowledged
event survives a crash. That's the shape of audit logs, write-ahead logs,
queue brokers and anything that promises "acknowledged means durable".
The one setting is `SYNC`, how the service makes each event durable:

- `SYNC=each` (bad, default): write the line, `fsync()`, reply. One fsync
  per request, one request at a time, behind a mutex.
- `SYNC=group` (good): requests queue their line and wait. A single
  writer writes everything queued so far and covers it with one
  `fsync()`, then replies to all of them. Requests that arrive during an
  fsync form the next batch. This is group commit.

Both settings reply only after an fsync has covered the event, so the
durability guarantee is the same. Only the number of fsyncs changes.

An `fsync()` is not one disk write. The filesystem has to write the data,
update its own metadata, and flush the device's write cache, as several
device requests in sequence. Each is fast, but together they take a
fraction of a millisecond, and the next fsync can't start until this one
is done. With one fsync per request, that fixes the service's ceiling at
about 1 / fsync time requests/s, however many clients are waiting. CPU
stays mostly idle, and nothing fails.

## Setup

Start the analysis container if it isn't running, then this example,
from this directory:
```sh
docker compose -f ../../../tools/analysis/compose.yml up -d
docker compose up -d --build                  # SYNC=each
SYNC=group docker compose up -d               # switch setting (recreates the service)
```
- `blk01-ingest` - the service on `:8080`. The log is
  `/data/events.log`, on a named volume rather than the container's
  writable layer. It logs every 5s: events/s, fsyncs/s, and events per
  fsync.

It's on `labnet`, so `lab-analysis` can load it with `wrk`. Load used in
both runs, 30s with 64 connections, each posting a ~200-byte JSON event
([post.lua](./post.lua)):
```sh
docker exec lab-analysis wrk -t2 -c64 -d30s --latency \
  -s /workspace/labs/ig-profile_blockio/examples/01-fsync-per-write/post.lua http://blk01-ingest:8080
```
`profile_blockio` covers the whole host, so nothing else should be doing
disk I/O during the runs. Here, the idle host did ~230 device requests in
10s. Observation commands used below, run while `wrk` is running:
```sh
# the service's own view
docker logs --tail 4 blk01-ingest
# device latency histogram for 10s, empty buckets removed
docker exec lab-analysis sh -c "ig run profile_blockio:latest --map-fetch-interval 10s -t 11 \
  | grep -v ' 0  *|'"
# total device requests in that 10s
docker exec lab-analysis ig run profile_blockio:latest --map-fetch-interval 10s -t 11 \
  | awk '$4==":"{s+=$5} END{print s}'
# device requests in 10s by container, process and direction
docker exec lab-analysis sh -c "ig run top_blockio:latest --map-fetch-interval 10s -t 11 -o json \
  | jq -r 'group_by([.runtime.containerName, .proc.comm, .rw])
      | map([(.[0].runtime.containerName | if . == \"\" then \"-\" else . end), .[0].proc.comm, .[0].rw, (map(.io) | add)])
      | sort_by(-.[3]) | .[] | @tsv'"
```

## Bad setting: `SYNC=each`

```sh
docker compose up -d --build
docker exec lab-analysis wrk -t2 -c64 -d30s --latency \
  -s /workspace/labs/ig-profile_blockio/examples/01-fsync-per-write/post.lua http://blk01-ingest:8080
```
producing output:
```
Running 30s test @ http://blk01-ingest:8080
  2 threads and 64 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    47.29ms   17.92ms 285.16ms   87.04%
    Req/Sec   692.94    184.63     1.20k    77.93%
  Latency Distribution
     50%   48.31ms
     75%   51.54ms
     90%   55.87ms
     99%  112.48ms
  41479 requests in 30.08s, 2.53MB read
Requests/sec:   1379.07
Transfer/sec:     86.19KB
```
The service's log and `docker stats --no-stream blk01-ingest` during the
run:
```
2026/10/09 16:23:18 t=  15s events/s=   1283 fsyncs/s=  1283 events/fsync=   1.0
2026/10/09 16:23:23 t=  20s events/s=   1271 fsyncs/s=  1271 events/fsync=   1.0
2026/10/09 16:23:28 t=  25s events/s=   1217 fsyncs/s=  1217 events/fsync=   1.0
2026/10/09 16:23:33 t=  30s events/s=   1256 fsyncs/s=  1256 events/fsync=   1.0

CONTAINER ID   NAME           CPU %     MEM USAGE / LIMIT     MEM %     NET I/O           BLOCK I/O        PIDS
49a6605cf517   blk01-ingest   16.45%    10.01MiB / 11.74GiB   0.08%     14.4MB / 7.11MB   7.44MB / 326MB   11
```
About 1,300 requests/s at a median of 48ms, to append 200 bytes to a
file, with the CPU at 16%. Nothing errors, nothing is saturated that a
dashboard would show, and the latency is all waiting. `docker stats`
shows the container writing to disk, but not whether the disk is slow or
just busy.

### Detecting it with `profile_blockio`

The first question is whether the disk is slow. Take a 10s histogram
while the load runs:
```
latency
        µs               : count    distribution
         2 -> 4          : 12835    |*********************************       |
         4 -> 8          : 12366    |********************************        |
         8 -> 16         : 11873    |******************************          |
        16 -> 32         : 1099     |**                                      |
        32 -> 64         : 327      |                                        |
        64 -> 128        : 186      |                                        |
       128 -> 256        : 9534     |************************                |
       256 -> 512        : 15347    |****************************************|
       512 -> 1024       : 927      |**                                      |
      1024 -> 2048       : 74       |                                        |
      2048 -> 4096       : 31       |                                        |
      4096 -> 8192       : 6        |                                        |
      8192 -> 16384      : 2        |                                        |
```
It isn't. Almost every request completes in under 512µs, in two peaks
(2-16µs and 128-512µs), and only a few hundred out of 64,000 take more
than a millisecond. A request that waits 48ms isn't waiting on a slow
device.

The second question is how many device requests there are, compared with
the work done. The total for the window:
```
64607
```
That's ~6,500 device requests/s against ~1,300 HTTP requests/s, about 5
per request. The service's own counters say it does one fsync per
request, so each fsync is about 5 device requests. Done one after
another, a few of them in the 128-512µs peak and the rest in the fast
one, they add up to roughly the ~0.75ms per fsync that a ceiling of
~1,300 fsyncs/s implies. The 48ms median is then mostly queueing: 64 connections waiting
on the mutex for one fsync each, 64 × 0.75ms ≈ 48ms.

`profile_blockio` can't say who issued those requests. `top_blockio`
can:
```
blk01-ingest	ingest	write	37223
-		read	108
blk01-ingest	ingest	read	13
```
Nearly all the I/O comes from `ingest` in `blk01-ingest`.
(`top_blockio` counts fewer requests than the histogram, 37,000 against
64,000, so it doesn't count every request type the histogram does. Use it
for attribution, not for the total.)

So the device is fast and the load is the problem: a fixed number of
device requests per HTTP request, issued one after another. A device
request rate that tracks the request rate means a synchronous disk round
trip on the request path.

## Good setting: `SYNC=group`

```sh
SYNC=group docker compose up -d
docker exec lab-analysis wrk -t2 -c64 -d30s --latency \
  -s /workspace/labs/ig-profile_blockio/examples/01-fsync-per-write/post.lua http://blk01-ingest:8080
```
producing output:
```
Running 30s test @ http://blk01-ingest:8080
  2 threads and 64 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     2.55ms    2.72ms  46.73ms   94.22%
    Req/Sec    14.63k     3.91k   21.49k    83.50%
  Latency Distribution
     50%    1.87ms
     75%    2.49ms
     90%    3.89ms
     99%   15.63ms
  875554 requests in 30.10s, 53.44MB read
Requests/sec:  29088.55
Transfer/sec:      1.78MB
```
and the service's log:
```
2026/10/09 16:23:52 t=  15s events/s=  31448 fsyncs/s=   1102 events/fsync=  28.5
2026/10/09 16:23:57 t=  20s events/s=  31700 fsyncs/s=   1118 events/fsync=  28.3
2026/10/09 16:24:02 t=  25s events/s=  31893 fsyncs/s=   1123 events/fsync=  28.4
2026/10/09 16:24:07 t=  30s events/s=  30915 fsyncs/s=   1107 events/fsync=  27.9
```
Same disk, same durability guarantee, and 29,000 requests/s instead of
1,400: about 21× the throughput, with the median latency down from 48ms
to 1.9ms. The fsync rate barely changed, ~1,100/s against ~1,300/s,
because fsync time is still the limit. Each fsync now covers ~28 events.

The histogram during the run:
```
latency
        µs               : count    distribution
         2 -> 4          : 1964     |****                                    |
         4 -> 8          : 16002    |****************************************|
         8 -> 16         : 9276     |***********************                 |
        16 -> 32         : 3008     |*******                                 |
        32 -> 64         : 2842     |*******                                 |
        64 -> 128        : 440      |*                                       |
       128 -> 256        : 7323     |******************                      |
       256 -> 512        : 11365    |****************************            |
       512 -> 1024       : 3450     |********                                |
      1024 -> 2048       : 193      |                                        |
      2048 -> 4096       : 25       |                                        |
      8192 -> 16384      : 1        |                                        |
```
with a total of:
```
55889
```
The shape is about the same as before, and the total is *lower*: ~5,600
device requests/s against ~6,500. The disk does about the same amount of
work in both runs, and still ~5 requests per fsync. What changed is how
much each request achieves: ~31,000 events/s over ~5,600 device
requests/s is 0.18 device requests per HTTP request, against ~5 before,
about 28× fewer. `top_blockio` still attributes it to the same process:
```
blk01-ingest	ingest	write	34733
blk01-ingest	ingest	read	61
-		write	42
```

This is why the device request count alone doesn't identify the problem:
the bad and good settings keep the disk about equally busy. The ratio of
device requests to application requests does.

## Takeaway

An fsync is several device requests in sequence, here ~5 and ~0.75ms, so
one fsync per request caps a service at about 1 / fsync time requests/s,
however fast the disk is and however idle the CPU. `profile_blockio`
shows it from outside the process: a histogram that says the device is
fast, and a device request rate that's a fixed multiple of the request
rate. Fix it with group commit: one fsync shared by all the requests
waiting for it, which keeps "acknowledged means durable" and lets
throughput grow with concurrency. Disabling fsync would also be fast, but
it gives up the guarantee.

## Tear down

```sh
docker compose down -v
```
