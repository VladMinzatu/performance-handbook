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

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down -v
```

