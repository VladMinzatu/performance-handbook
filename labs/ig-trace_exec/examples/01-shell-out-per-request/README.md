# Shelling out per request

A checksum service, `GET /checksum?id=N`, that returns the SHA-256 of a
4KB document. It's the common case of application code calling a CLI tool
for something a library could do. The one setting is `MODE`:

- `MODE=shell` (bad, default): each request runs `sh -c sha256sum` with
  the document on stdin. It's the shape of `subprocess.run(..., shell=True)`,
  `os.system()`, or `exec.Command("sh", "-c", ...)`.
- `MODE=native` (good): the same hash, computed in-process with
  `crypto/sha256`.

Each process start costs a `fork`/`clone`, an `execve`, a dynamic-loader
run and an exit. That's about a millisecond of CPU, against the
microseconds of the actual work. Going through `sh -c` doubles it: dash
forks again to run `sha256sum` instead of replacing itself. None of that
time is spent in the service's own code, and the processes live too
briefly for `ps` or `top` to show a consistent picture. Expect `MODE=shell`
to run two execs per request and to reach its 1-CPU limit at a few hundred
requests/s, where `MODE=native` reaches tens of thousands.

## Setup

Start the analysis container if it isn't running, then this example,
from this directory:
```sh
docker compose -f ../../../tools/analysis/compose.yml up -d
docker compose up -d --build                  # MODE=shell
MODE=native docker compose up -d              # switch to the good setting
```
- `exec01-checksum` - the service on `:8080`, limited to 1 CPU, on
  `labnet` so `lab-analysis` can load it with `wrk`.

Load used in both runs (30s, 16 connections):
```sh
docker exec lab-analysis wrk -t2 -c16 -d30s --latency 'http://exec01-checksum:8080/checksum?id=42'
```

## Bad setting: `MODE=shell`

```sh
docker compose up -d --build
docker exec lab-analysis wrk -t2 -c16 -d30s --latency 'http://exec01-checksum:8080/checksum?id=42'
```
producing output:
```
Running 30s test @ http://exec01-checksum:8080/checksum?id=42
  2 threads and 16 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    33.50ms   31.59ms 190.36ms   72.30%
    Req/Sec   295.71    118.79     0.95k    85.47%
  Latency Distribution
     50%   21.63ms
     75%   59.78ms
     90%   83.85ms
     99%   94.88ms
  17686 requests in 30.05s, 3.07MB read
Requests/sec:    588.58
Transfer/sec:    104.61KB
```
While it runs, from another terminal:
```sh
docker stats --no-stream exec01-checksum
```
producing output:
```
CONTAINER ID   NAME              CPU %     MEM USAGE / LIMIT     MEM %     NET I/O          BLOCK I/O     PIDS
b3280a790e5e   exec01-checksum   98.98%    13.03MiB / 11.74GiB   0.11%     454kB / 1.01MB   2.94MB / 0B   43
```
The service is pinned at its 1-CPU limit while serving under 600 req/s,
about 1.7ms of CPU per request to hash 4KB. `PIDS 43` is the first clue:
a single Go server process shouldn't need dozens of tasks. Snapshots like
this (or `docker top`) catch a different set of short-lived children
each time, and give neither a rate nor a cause.

### Detecting it with `trace_exec`

Watch the execs as they happen:
```sh
docker exec lab-analysis sh -c "ig run trace_exec:latest -c exec01-checksum -t 2 -o json \
  | jq -r '[.proc.parent.comm, .proc.comm, .args] | @tsv' | head -6"
```
producing output:
```
sh	sha256sum	sha256sum
checksum	sh	sh -c sha256sum
checksum	sh	sh -c sha256sum
sh	sha256sum	sha256sum
checksum	sh	sh -c sha256sum
sh	sha256sum	sha256sum
```
Each line reads as parent, then the program started, then its command
line. The server (`checksum`) starts `sh -c sha256sum`, and each `sh`
then starts `sha256sum`: a two-level process tree for every hash. The
command line points straight at the code to look for.

Then measure the rate:
```sh
docker exec lab-analysis sh -c "ig run trace_exec:latest -c exec01-checksum -t 10 -o json \
  | jq -r '[.proc.parent.comm, .proc.comm] | @tsv' | sort | uniq -c"
```
producing output:
```
   6141 checksum	sh
   6140 sh	sha256sum
```
That's ~614 of each per second, one pair per request at the ~590 req/s
`wrk` measured. An exec rate that tracks the request rate means the
process creation is on the request path. The `sh` level is pure
overhead: half the execs exist only to parse a command line that has no
shell syntax in it.

## Good setting: `MODE=native`

```sh
MODE=native docker compose up -d
docker exec lab-analysis wrk -t2 -c16 -d30s --latency 'http://exec01-checksum:8080/checksum?id=42'
```
producing output:
```
Running 30s test @ http://exec01-checksum:8080/checksum?id=42
  2 threads and 16 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     8.12ms   14.51ms  86.54ms   84.27%
    Req/Sec    12.10k     5.18k   38.83k    85.19%
  Latency Distribution
     50%  318.00us
     75%    7.50ms
     90%   33.15ms
     99%   56.65ms
  721618 requests in 30.01s, 125.25MB read
Requests/sec:  24045.94
Transfer/sec:      4.17MB
```
and `docker stats --no-stream exec01-checksum` during the run:
```
CONTAINER ID   NAME              CPU %     MEM USAGE / LIMIT     MEM %     NET I/O           BLOCK I/O     PIDS
4c8bede9805e   exec01-checksum   101.59%   9.352MiB / 11.74GiB   0.08%     17.1MB / 33.7MB   69.6kB / 0B   17
```
Same CPU limit, same work, and 24,000 req/s instead of 590: about 40×,
or ~40µs of CPU per request instead of ~1.7ms. `PIDS` drops to 17, the
Go runtime's own threads. (The p75-p99 here comes from CFS throttling at
the 1-CPU limit under a closed-loop load, not from the hash.)

The same 10s count:
```sh
docker exec lab-analysis sh -c "ig run trace_exec:latest -c exec01-checksum -t 10 -o json \
  | jq -r '[.proc.parent.comm, .proc.comm] | @tsv' | sort | uniq -c"
```
produces no output: zero execs at 24,000 req/s.

## Takeaway

A process start costs about a millisecond of CPU, and `sh -c` doubles
it, so per-request exec caps a service at a few hundred requests per CPU
whatever the work itself costs. The signature is an exec rate in
`trace_exec` that tracks the request rate, with the parent → child
chain and command line pointing at the call site. Fix it in-process (a
library, or a long-lived worker process fed over a pipe). Failing that,
exec the tool directly without `sh -c`, which removes half the execs.

## Tear down

```sh
docker compose down
```
