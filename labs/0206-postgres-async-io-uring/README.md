# Postgres async I/O: io_uring vs worker processes vs synchronous reads

Uses the shared lab infrastructure in [tools/](../tools/README.md) - the
`analysis` container is needed for the OS-level tracing steps (comparing
what each `io_method` actually does at the syscall level), not just for
running queries.

## Background

Every Postgres release before 18 reads a heap or index page the same way:
a backend calls a blocking `pread()`, and that backend does nothing else
until the kernel returns the page. One backend, one I/O, one syscall at a
time - fine when the page is already in the OS page cache (`pread()`
returns almost immediately), but a real stall when it isn't, since the
backend can't do anything useful, not even start reading the *next* page
it already knows it'll need, until the current one lands.

Postgres 18 introduces a genuine AIO subsystem, controlled by `io_method`:
- `sync` - the old behavior, kept as the safe default.
- `worker` - reads are handed off to a small pool of dedicated I/O worker
  processes; the requesting backend can move on to other work (or queue
  more reads) while a worker does the blocking `pread()` on its behalf.
- `io_uring` (Linux only, requires `liburing` at build time) - the
  backend itself submits reads into an `io_uring` submission queue and
  reaps completions from a completion queue, with no separate worker
  process or inter-process handoff involved at all.

All three exist to let Postgres have more than one read *in flight* at a
time - the same "don't block on one thing when you could have several
outstanding" idea behind readahead and prefetching generally, just
applied at the storage-I/O layer instead of the query-planning layer.
`effective_io_concurrency` and `maintenance_io_concurrency` are what
actually tell Postgres how many reads to keep in flight (bitmap heap
scans and sequential prefetching already respect these); `io_method`
only changes the *mechanism* used to issue them.

## Hypotheses

**Prediction 1 - `sync` should be measurably worse than `worker` and
`io_uring` specifically on I/O-bound, multi-page access patterns, and
show little to no difference on access patterns that don't benefit from
concurrency.** A bitmap heap scan or sequential scan over a table larger
than `shared_buffers`, forced cold (OS cache dropped/bypassed), should
run faster under `worker` or `io_uring` than under `sync`, because both
let Postgres have several page reads outstanding at once instead of
strictly one-at-a-time. A single-row index lookup, which only ever needs
one page, should show no real difference between methods - there's
nothing to overlap.

**Prediction 2 - the mechanism difference is directly visible at the
syscall level, not just in wall-clock time.** `sync` should show one
`pread64` per page, one at a time, on the querying backend itself.
`worker` should show those `pread64` calls happening on separate
`postgres: io worker` OS processes, with the querying backend visibly
doing something else (or waiting on IPC) in between. `io_uring` should
show `io_uring_enter` calls on the backend itself - batches of
submissions and completions - and no separate worker processes doing the
reading at all. This should be checkable purely from process/syscall
tracing, without needing to trust `EXPLAIN`'s own timing.

**Prediction 3 - `io_uring`'s advantage over `worker` should show up in
CPU/context-switch cost, not necessarily raw I/O latency.** Both methods
let reads overlap, so Prediction 1's throughput win might land similarly
for either. But `worker` pays for that overlap with real inter-process
communication - the backend has to hand off a request and later be
woken up by a worker - while `io_uring` keeps everything in the
requesting backend's own address space, submitted and reaped without a
second process involved. At a high enough concurrent read rate,
`worker`'s context-switch rate (backend <-> io worker) should be
noticeably higher than `io_uring`'s for the same amount of I/O
completed.

**Prediction 4 (stretch, environment-dependent) - the whole comparison
may be muted if the underlying storage never actually queues.** All of
this only matters if there's a real, non-trivial gap between "one I/O in
flight" and "several I/Os in flight" - true of a real disk with queue
depth, potentially not true of a virtual disk backed by a fast host-side
cache, where even a blocking `pread()` might return fast enough that
overlapping reads buys little. Worth checking early rather than assuming
- if Predictions 1-3's *syscall-level* differences hold but the
wall-clock gap is small, that's the likely explanation, not a failure
of the AIO subsystem itself.

## Setup

Start Postgres (18, the first version with `io_method`) - `compose.yml`
already sets `security_opt: seccomp:unconfined`, which `io_method=io_uring`
needs (Docker's default seccomp profile blocks the `io_uring_*` syscalls
outright, otherwise):
```sh
docker compose -f compose.yml up -d
```

Confirm `io_uring` is actually available before relying on it - support
depends on how the binary was built (Linux + `liburing` at compile time),
not just the Postgres version:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT unnest(enumvals) FROM pg_settings WHERE name = 'io_method';"
```

Load a table larger than `shared_buffers` (64MB, set in `compose.yml`)
so a scan can't be served entirely from Postgres's own cache - 3M padded
rows, ~750MB:
```sh
docker cp seed.sql lab-postgres:/tmp/seed.sql
docker exec lab-postgres psql -U postgres -d labdb -f /tmp/seed.sql
```

`io_method` requires a restart to change (it's not reloadable), so
switching between the three settings goes through `compose.yml`'s
`command:`/config rather than `SET`:
```sh
# edit io_method in compose.yml, then:
docker compose -f compose.yml up -d --force-recreate postgres
```

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down -v
```
