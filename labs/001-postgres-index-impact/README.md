# 001 - Postgres index impact

Uses the shared lab infrastructure in [tools/](../tools/README.md).

## Hypothesis

`orders` has no index beyond the primary key, and we run a highly selective
lookup: `WHERE customer_id = X`, matching ~10 rows out of 5,000,000
(~0.0002% of the table). With no usable index, the planner has no choice but
a sequential scan.

**Prediction:**
1. Adding a B-tree index on `customer_id` will flip the plan from `Seq Scan`
   to `Index Scan`, and cut buffer reads and latency for that query by
   orders of magnitude.
2. That benefit isn't free: once the index exists, `INSERT` throughput will
   drop measurably, because Postgres now has to maintain the index (an
   extra B-tree page write) on every insert, not just append a heap row.

The experiment tests both halves - the read speedup and the write cost.

## Setup

One-time, if not already running - start the analysis container:
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build
```

Start Postgres for this lab:
```sh
docker compose -f compose.yml up -d
```

Load the seed data (~5M rows, takes a minute or two):
```sh
docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql
```

Pick a `customer_id` to query throughout the experiment - any value should
do, but grab one that actually has a handful of rows so the plan/timing
differences aren't due to an empty result set:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT customer_id, count(*) FROM orders GROUP BY customer_id ORDER BY random() LIMIT 1;"
```
Use that value (call it `$CID` below) consistently in the steps that follow.

## Step 1 - baseline (no index)

Run the query with `EXPLAIN (ANALYZE, BUFFERS)`:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM orders WHERE customer_id = $CID;"
```
What to check:
- Plan node: should say `Seq Scan on orders`.
- `Buffers: shared hit=... read=...` - this is pages actually touched; with
  a ~5M row table this should be a large fraction of the table.
- `Execution Time`.

Also check `pg_stat_user_tables` before/after a few runs - `seq_scan` should
increment, `idx_scan` should stay at 0 (or unchanged) for this table:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT seq_scan, seq_tup_read, idx_scan FROM pg_stat_user_tables WHERE relname = 'orders';"
```

Optional, to see it at the OS level: get a shell in the analysis container
(`docker compose -f ../tools/analysis/compose.yml exec analysis bash`),
find the backend PID handling your session
(`SELECT pg_backend_pid();` in a `psql` session you keep open), and in a
second analysis shell run `strace -c -p <pid>` or a bpftrace one-liner on
`block:block_rq_issue`/`syscalls:sys_enter_pread64` while you fire the query
from the kept-open session - you should see a burst of reads proportional
to the sequential scan.

## Step 2 - add the index, repeat the read test

```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "CREATE INDEX CONCURRENTLY idx_orders_customer_id ON orders (customer_id);"
```

Re-run the same `EXPLAIN (ANALYZE, BUFFERS)` and `pg_stat_user_tables` /
`pg_stat_user_indexes` checks as Step 1:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM orders WHERE customer_id = $CID;"

docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT idx_scan, idx_tup_read FROM pg_stat_user_indexes WHERE indexrelname = 'idx_orders_customer_id';"
```
What to check:
- Plan node should now say `Index Scan using idx_orders_customer_id`.
- `Buffers` should be a handful of pages instead of most of the table.
- `Execution Time` should have dropped by (roughly) orders of magnitude.
- `idx_scan` on the index should now be incrementing.

## Step 3 - the write-side cost

Measure INSERT throughput/latency with `pgbench`, before and after the
index exists. Since the index from Step 2 is already there, do this
comparison as: drop the index, benchmark, recreate it, benchmark again.

Without the index:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "DROP INDEX idx_orders_customer_id;"

docker cp bench_insert.sql lab-postgres:/tmp/bench_insert.sql
docker exec lab-postgres pgbench -n -f /tmp/bench_insert.sql -c 4 -j 2 -T 30 -U postgres labdb
```

With the index:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "CREATE INDEX CONCURRENTLY idx_orders_customer_id ON orders (customer_id);"

docker exec lab-postgres pgbench -n -f /tmp/bench_insert.sql -c 4 -j 2 -T 30 -U postgres labdb
```
What to check: `pgbench`'s own summary gives you `tps` and average latency
directly - compare the two runs. If you want to see *why*, `EXPLAIN
(ANALYZE, BUFFERS)` on a single `INSERT` (wrap in a transaction and roll it
back so you don't skew the table) will show the extra index-maintenance
work once the index exists.

## Tear down

```sh
docker compose -f compose.yml down -v
```
(`analysis` can stay running for the next lab.)
