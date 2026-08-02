# 005 - Postgres connection overhead and pooling

Uses the shared lab infrastructure in [tools/](../tools/README.md). The
`analysis` container is only needed for the optional OS-level stretch step
- the core lab is `psql`, `pgbench`, and PgBouncer.

## Background

Postgres uses one OS process per connection: every new connection means the
postmaster `fork()`s a fresh backend process, which then has to attach to
shared memory and set up its own local state before it can run a single
query. On top of that, establishing the connection itself costs a TCP
handshake and an authentication exchange (SCRAM, here, since a password is
configured). None of this shows up in a query's own `EXPLAIN ANALYZE` -
it's entirely separate, paid-once-per-connection overhead that sits in
front of the query.

This is easy to miss for the same reason N+1 round trips are easy to miss:
it's a fixed tax that's invisible when connections are long-lived and
reused (a typical app-server connection pool, or a single interactive
`psql` session), and severe the moment something opens a fresh connection
per unit of work - a naive script, a serverless function with no warm
pool, or a misconfigured application-level pool that's sized too small or
recycles connections too aggressively.

The standard fix is a connection pooler sitting in front of Postgres -
here, [PgBouncer](https://www.pgbouncer.org/) in `transaction` pooling
mode, which hands a real Postgres backend connection to a client only for
the duration of one transaction, then returns it to the pool for the next
client. From the application's point of view it still "connects" for every
unit of work; from Postgres's point of view, the actual backend processes
stay few, warm, and reused.

## Hypotheses

**Prediction 1 - connection setup is a real, fixed, measurable cost,
separate from query execution.** Running a trivial `SELECT 1` on a brand
new connection should take noticeably longer, wall-clock, than running the
same query on an already-open session - and that gap should track with
connection setup specifically (TCP + auth + backend fork), not anything
query-related, since the query itself does effectively no work either way.

**Prediction 2 - this fixed cost dominates throughput for connect-per-
operation workloads, even for trivial queries.** `pgbench`'s `-C` mode
(open a fresh connection for every transaction) running nothing but
`SELECT 1` should show dramatically lower `tps` than the same script run
over persistent, reused connections (`pgbench`'s default) - despite the
SQL work being identical and negligible in both cases. If this gap is
large even for a query this trivial, it's entirely attributable to
connection overhead, not anything happening inside Postgres's executor.

**Prediction 3 - a pooler recovers most of that lost throughput, with no
application changes.** Repeating Prediction 2's connect-per-transaction
workload, but pointed at PgBouncer instead of Postgres directly, should
recover a large fraction of the lost throughput - because PgBouncer
absorbs the client-visible "new connection" cheaply (it's already holding
warm backend connections open) even though the client still believes it's
opening a fresh connection every time.

**Prediction 4 - direct connections fail hard under a connection burst
past `max_connections`; pooled connections degrade instead.** Firing a
sudden burst of concurrent connect-per-transaction clients that exceeds
Postgres's configured `max_connections` directly at Postgres should
produce outright rejections (`sorry, too many clients already`) for the
excess. The identical burst aimed at PgBouncer - configured with a much
higher client-facing limit but a small, fixed number of real backend
connections - should be absorbed instead: requests queue for a moment
waiting for a pooled backend, but nothing gets rejected.

**Prediction 5 (stretch) - the backend-fork mechanism behind all of this
is directly observable at the OS level.** Tracing process-fork events on
the Postgres postmaster should show roughly one new backend process per
connection for direct, connect-per-transaction traffic, but a much lower,
decoupled rate for the same client-side load routed through PgBouncer in
transaction-pooling mode - since a small, fixed set of already-forked
backends is being shared and reused rather than spun up fresh each time.

## Setup

Start Postgres and PgBouncer together:
```sh
docker compose -f compose.yml up -d
```
`max_connections` is deliberately lowered to 30 (via `compose.yml`) so
Prediction 4's burst is easy to trigger without needing hundreds of
concurrent clients. PgBouncer is configured with `pool_mode = transaction`,
`max_client_conn = 1000` (client-facing - generous), and
`default_pool_size = 20` (backend-facing - comfortably under Postgres's
30), listening on port 6432.

No seed data is needed - this lab is about connection cost, not query
cost, so the query used throughout is as close to free as possible:
```sh
docker cp select1.sql lab-postgres:/tmp/select1.sql
```

PgBouncer requires real SCRAM authentication on both sides (matching
Postgres's own default once a password is configured), so any command
aimed at it - unlike the direct-to-Postgres commands elsewhere in this
handbook, which rely on the local Unix socket's `trust` auth - needs a
password supplied explicitly:
```sh
docker exec -e PGPASSWORD=postgres lab-postgres psql -h lab-pgbouncer -p 6432 -U postgres -d labdb -c "SELECT 1;"
```

To start an interactive terminal (not via pgbouncer):
```
docker exec -it lab-postgres psql -U postgres -d labdb
```

## Step 1 - connection setup cost in isolation (prediction 1)

```sh
docker exec lab-postgres psql -U postgres -d labdb -c '\timing' -c 'SELECT 1;'
```
compared against timing just the query on an already-open session (open
one interactive session, run `\timing`, then `SELECT 1;` a few times in a
row - the first may still show setup cost, subsequent ones shouldn't).
What to check: the one-shot invocation's total time versus the warmed-up
session's per-query time - the difference is connection overhead, not
query time, since the query is identical in both cases.

## Step 2 - connect-per-transaction vs. persistent, throughput (prediction 2)

```sh
# persistent connections (pgbench's default - connect once, reuse)
docker exec lab-postgres pgbench -n -c 10 -j 4 -T 10 -f /tmp/select1.sql -U postgres labdb

# connect-per-transaction
docker exec lab-postgres pgbench -n -C -c 10 -j 4 -T 10 -f /tmp/select1.sql -U postgres labdb
```
What to check: `tps` for both. Both point at Postgres directly, at a
concurrency (10) comfortably under `max_connections` (30), so this isolates
connection-churn cost by itself, with no contention effects mixed in.

## Step 3 - pooling recovers throughput (prediction 3)

Same connect-per-transaction workload as Step 2, this time through
PgBouncer:
```sh
docker exec -e PGPASSWORD=postgres lab-postgres pgbench -n -C -c 10 -j 4 -T 10 \
  -h lab-pgbouncer -p 6432 -f /tmp/select1.sql -U postgres labdb
```
What to check: `tps` here versus Step 2's direct connect-per-transaction
number - expect most (not all - PgBouncer adds its own small proxying
overhead) of the gap from Step 2 to close, with no code/query changes at
all, only the connection target.

## Step 4 - bursty traffic: rejection vs. graceful degradation (prediction 4)

A burst of concurrent connect-per-transaction clients past
`max_connections=30`, direct to Postgres:
```sh
docker exec lab-postgres pgbench -n -C -c 40 -j 8 -T 5 -f /tmp/select1.sql -U postgres labdb
```
What to check: expect `pgbench` to abort with
`FATAL: sorry, too many clients already` partway through - a hard failure,
not a slowdown.

The identical burst through PgBouncer:
```sh
docker exec -e PGPASSWORD=postgres lab-postgres pgbench -n -C -c 40 -j 8 -T 5 \
  -h lab-pgbouncer -p 6432 -f /tmp/select1.sql -U postgres labdb
```
What to check: this run should complete without errors - higher latency
than Step 3 (clients now queue for one of the pool's 20 backend slots
under this heavier load) but no rejections, unlike the direct case.

## Step 5 (stretch) - watching backend forks at the OS level (prediction 5)

From the `analysis` container, trace new backend processes being spawned
by the postmaster while re-running Step 2's connect-per-transaction burst
directly, then Step 4's burst through PgBouncer:
```sh
PG_PID=$(docker inspect -f '{{.State.Pid}}' lab-postgres)
docker compose -f ../tools/analysis/compose.yml exec -T analysis bash -c "
  bpftrace -e '
    tracepoint:sched:sched_process_fork
    /comm == \"postgres\"/
    { @forks = count(); }
    interval:s:1 { print(@forks); clear(@forks); }
    interval:s:12 { exit(); }
  '
"
```
(run the direct and pooled traffic in a second shell while this is
running). What to check: a fork count roughly tracking the direct run's
transaction rate, versus a much lower, flatter fork count while the same
client-side load goes through PgBouncer - the same backend processes are
being reused across many client "connections" instead of one fork per
connection.

## Tear down

```sh
docker compose -f compose.yml down -v
```
