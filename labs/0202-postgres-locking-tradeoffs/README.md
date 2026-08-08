# 002 - Postgres locking tradeoffs: optimistic vs pessimistic

Uses the shared lab infrastructure in [tools/](../tools/README.md) (even 
if the `analysis` container isn't required for the core lab here, we still 
need to start it to set up the `labnet` network). But most things here are
observable through plain SQL and `pgbench`.

## Background

Whenever more than one transaction might update the same row, there are two
common ways applications handle it:

- **Pessimistic locking**: `SELECT ... FOR UPDATE` before doing anything
  else. This takes a real row lock immediately, so any other transaction
  that wants the same row simply waits until the first one commits or rolls
  back. Simple to reason about, but contention turns directly into queueing
  delay.
- **Optimistic locking**: read the row (including a `version` column)
  without locking it, do whatever work/thinking the application needs, then
  `UPDATE ... WHERE id = :id AND version = :version_read`. If another
  transaction updated the row first, this affects 0 rows - no error, no
  lock wait, just silence. The application is expected to check the
  affected row count and retry. If it doesn't, that's a real, quiet lost
  update, not just a performance quirk.

This is one of the most common concurrency decisions in application code -
decrementing inventory, adjusting an account balance, claiming a booking
slot - and the two approaches fail in qualitatively different ways under
load, which is what this lab measures.

Both scripts below simulate "think time" via `pg_sleep` between the read
and the write, standing in for whatever application-side work would
normally happen there (validation, business logic, a second query). Without
some think time, contention barely shows up at all since read-then-write
happens in microseconds.

## Hypotheses

**Prediction 1 - under low contention, both perform about the same.**
With many distinct rows and few clients per row, collisions are rare enough
that pessimistic's locking overhead and optimistic's (occasional) lost
updates should both be minimal.

**Prediction 2 - under high contention, pessimistic trades throughput for
correctness.** With many clients hammering the *same* row, pessimistic
locking should show throughput dropping and per-transaction latency rising
roughly with the number of clients queued behind the lock - but the final
balance should always exactly match the number of transactions processed:
no lost updates, ever.

**Prediction 3 - under the same high contention, optimistic locking trades
correctness for throughput.** Per-statement latency should stay low (no
blocking), but a growing fraction of updates should silently affect 0 rows
as the version moves out from under them between read and write. Without
retry logic, this is measurable directly as a gap between the number of
transactions `pgbench` reports as processed and the actual change in total
balance - that gap *is* the lost-update count.

**Prediction 4 (stretch) - longer think time makes each failure mode
worse, in its own way.** Increasing `pg_sleep` duration should increase
pessimistic's average latency roughly proportionally (longer lock hold
time under the same concurrency), while increasing optimistic's lost-update
rate (a longer window between read and write is more time for someone else
to get there first).

## Setup

Start Postgres for this lab:
```sh
docker compose -f compose.yml up -d
```

Load the seed table (1000 rows, each starting at `balance = 1000000`,
`version = 0`):
```sh
docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql
```

Copy the two `pgbench` scripts in:
```sh
docker cp pessimistic.sql lab-postgres:/tmp/pessimistic.sql
docker cp optimistic.sql lab-postgres:/tmp/optimistic.sql
```

Both scripts take two variables via `pgbench -D`:
- `range` - contention level. `range=1` means every client fights over a
  single row; `range=1000` spreads clients across the whole table.
- `think_time` - seconds of simulated app-side work between the read and
  the write.

**Re-run `seed.sql` before every individual measurement below** - it resets
all balances/versions to a known baseline, so "expected vs actual
decrement" is clean and specific to that one run.

To check for lost updates after a run, compare `pgbench`'s own summary
(`number of transactions actually processed`) against the real change in
total balance:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT 1000 * 1000000 - SUM(balance) AS actual_decrement FROM hot_accounts;"
```
For pessimistic runs this should equal the transaction count exactly. For
optimistic runs under contention, it should come in lower - the difference
is the number of updates that were silently lost.

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down -v
```
