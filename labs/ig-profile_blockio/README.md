# `ig profile_blockio`

Inspektor Gadget's `profile_blockio` builds a latency histogram of block
device I/O: how long each request to a disk took, from the moment it was
issued to the device until it completed. Each example under
[examples/](./examples) is a small program that uses the disk badly, run
once with a bad setting and once with a good one, with `profile_blockio`
used to detect the bad case. Uses the shared lab infrastructure in
[tools/](../tools/README.md) (`lab-analysis` runs `ig`).

## What it sees

Every block I/O request that completes on the host goes into one
histogram with power-of-two buckets in microseconds. It's captured in the
kernel's block layer (`block_rq_issue` / `block_rq_complete`), so every
request is counted however fast it is. The histogram is printed once per
fetch interval (1s by default) and is cumulative: counts keep growing
from the moment the gadget starts.

10 seconds of a container doing 4KB synchronous direct writes in a loop
(empty buckets removed):
```
latency
        µs               : count    distribution
         1 -> 2          : 463      |**                                      |
         2 -> 4          : 5227     |***************************             |
         4 -> 8          : 3560     |******************                      |
         8 -> 16         : 6045     |*******************************         |
        16 -> 32         : 2107     |***********                             |
        32 -> 64         : 2317     |************                            |
        64 -> 128        : 1331     |******                                  |
       128 -> 256        : 7609     |****************************************|
       256 -> 512        : 4624     |************************                |
       512 -> 1024       : 3492     |******************                      |
      1024 -> 2048       : 1740     |*********                               |
      2048 -> 4096       : 270      |*                                       |
      4096 -> 8192       : 6        |                                        |
```
How to read it:
- Each row is a latency range, and `count` is how many requests finished
  in that range. `256 -> 512` means 256-511µs.
- The total count divided by the window is the device's request rate.
  Here that's 38,791 requests in 10s, about 3,900 IOPS.
- The shape matters more than any single number. Look at where the bulk
  sits, and how far out the tail goes. This one has two peaks, 2-16µs and
  128-1024µs, so its average (~250µs from bucket midpoints) describes
  almost no actual request.
- Bucket edges are a factor of 2 apart, so a shift of one row is a 2×
  change in latency.

What it doesn't see:
- Which container, process or disk. The histogram is host-wide: every
  device and every process together, and this version takes no `-c`
  filter. Run it with only the workload you care about doing I/O, or
  compare against a baseline taken just before. `top_blockio` shows I/O
  per process and container, and per device. Writeback done by kernel
  threads often shows up there with no container.
- I/O that never reaches the disk. Reads served from the page cache, and
  buffered writes before they're flushed, don't appear. A `write()` that
  returns quickly and is written back seconds later shows up at writeback
  time, not when the application called it.
- Time spent before the request is issued: waiting in the I/O scheduler's
  queue, in the filesystem (journal commits, locks), or in the
  application. Application-visible latency, such as an `fsync()` call,
  can be much longer than any single device request.
- Request size, type (read, write, flush) or offset. Different kinds of
  request mixed together show up as several peaks, and the gadget can't
  tell you which peak is which.
- Network filesystems (NFS, SMB, FUSE), which don't go through the block
  layer.

## Why it matters in production

When a service is slow on disk, there are two very different causes. The
storage is slow (high latency per request), or the service is asking for
too many requests (normal latency, high count). Disk metrics such as
utilisation and queue depth tend to show both as "the disk is busy".
`profile_blockio` separates them: the shape of the histogram is the
device's latency, and the count is the load. Typical sources:
- synchronous writes per operation: `fsync()` after every write,
  synchronous commits in a database, `O_SYNC`/`O_DSYNC` log files, each one
  a device round trip on the request path
- small random reads when the working set no longer fits in memory: a
  container memory limit that evicts the page cache, an index that has
  outgrown RAM, or a cache that was just restarted
- I/O patterns that break up large transfers into many small requests:
  small buffer sizes, unbuffered writes, `O_DIRECT` with small blocks
- a noisy neighbour: another container's bulk writes (backups, compaction,
  log shipping) moving everyone else's requests into the tail
- throttled cloud volumes: once a volume hits its IOPS or throughput cap,
  latency jumps from sub-millisecond to many milliseconds, with no change
  in the application
- swapping, which turns memory access into disk I/O

`profile_blockio` answers what the device is actually doing: how many
requests per second, how long each takes, and whether the tail has
moved.

## Usage

```sh
# one histogram per second, cumulative since start
docker exec lab-analysis ig run profile_blockio:latest
# one histogram for a 10s window (fetch once, just before the timeout)
docker exec lab-analysis ig run profile_blockio:latest --map-fetch-interval 10s -t 11
# the same, without the empty buckets
docker exec lab-analysis sh -c "ig run profile_blockio:latest --map-fetch-interval 10s -t 11 \
  | grep -v ' 0  *|'"
# who is doing the I/O: per process/container and device
docker exec lab-analysis ig run top_blockio:latest
```
There's no JSON output for this gadget, only the histogram. To compare a
bad setting with a good one, take a 10s histogram of each under the same
load and compare the total count and where the peaks are.

## Examples

1. [fsync per write](./examples/01-fsync-per-write/README.md) - an
   event-ingest service that fsyncs every event before replying, against
   group commit with the same durability.
