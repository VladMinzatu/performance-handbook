# 003 - Postgres transaction isolation levels

Uses the shared lab infrastructure in [tools/](../tools/README.md) (the
`analysis` container isn't required for the core lab here - the
demonstrations are plain SQL run across two interleaved sessions, but we still 
need to start it to set up the `labnet` network)

## Background

The SQL standard defines four isolation levels (READ UNCOMMITTED, READ
COMMITTED, REPEATABLE READ, SERIALIZABLE), each meant to forbid more kinds
of concurrency anomalies than the last. Postgres only really implements
three: it accepts `READ UNCOMMITTED` but treats it identically to `READ
COMMITTED`, because Postgres's MVCC design means a transaction can never
see another transaction's uncommitted rows in the first place - a dirty
read simply isn't possible at any level.

The three levels that do differ, from weakest to strongest guarantee:
- **READ COMMITTED** (the default) - each individual *statement* within a
  transaction gets its own fresh snapshot of the committed data.
- **REPEATABLE READ** - the whole *transaction* gets one snapshot, taken at
  its first statement; every later statement sees that same snapshot,
  regardless of what else commits in the meantime.
- **SERIALIZABLE** - everything REPEATABLE READ gives you, plus active
  monitoring for read/write patterns between concurrent transactions that
  couldn't have happened in *any* one-at-a-time ordering - and abortion of
  one of them (with a retryable error) if it finds one.

A common application pattern - optimistic locking via a version column,
`UPDATE ... WHERE version = :version` - can silently affect 0 rows when it
loses a race against a concurrent update, under READ COMMITTED. Isolation
level turns out to be *why* that failure is silent rather than an error,
and changing it changes the failure mode entirely.

## Hypotheses

**Prediction 1 - non-repeatable reads happen under READ COMMITTED, not
under REPEATABLE READ or SERIALIZABLE.** If transaction A reads a row,
transaction B updates and commits that same row, and A then reads it
again in the same transaction: under READ COMMITTED, A's second read sees
B's committed change. Under REPEATABLE READ/SERIALIZABLE, A's second read
still sees the original value - its snapshot was fixed at the start.

**Prediction 2 - a version-checked lost-update race fails silently under
READ COMMITTED, but becomes a real, retryable error under REPEATABLE
READ/SERIALIZABLE.** For an optimistic-locking `UPDATE ... WHERE version =
:version` race between two transactions: under READ COMMITTED, Postgres
re-evaluates the `WHERE version = :version` clause against the latest
committed row and just finds no match (0 rows affected, no error - a
silent lost update). Under REPEATABLE READ/SERIALIZABLE, Postgres instead
raises `ERROR: could not serialize access due to concurrent update`
(SQLSTATE `40001`) - the application gets a hard signal that something
needs to retry, instead of a result that looks like success.

**Prediction 3 - REPEATABLE READ still allows write skew; SERIALIZABLE
doesn't.** Two transactions can each independently read the same data,
each individually make a change that's valid given what they read, and
together violate an invariant that neither violated alone. Classic case:
an on-call rotation where "at least one doctor must be on call" - two
concurrent transactions each check "is at least one *other* doctor on
call?" (yes), and each takes themselves off call. Under REPEATABLE READ,
both commit successfully, leaving nobody on call. Under SERIALIZABLE, one
of the two gets a serialization failure.

**Prediction 4 (stretch) - stronger isolation trades silent data loss for
throughput lost to retries.** Running a high-contention hot-row workload
(many clients racing an optimistic-locking `UPDATE` against the same row)
under each isolation level should show: READ COMMITTED with high
throughput and silent lost updates; REPEATABLE READ/SERIALIZABLE with
measurably more `40001` errors as contention rises, and lower *effective* (successfully
committed) throughput once retries are accounted for - the cost of
correctness shows up as wasted, retried work rather than as blocking.

## Setup

Start Postgres for this lab:
```sh
docker compose -f compose.yml up -d
```

Load the seed data - a single-row `accounts` table (with a `version`
column for the optimistic-locking pattern) for the non-repeatable-read/
lost-update demos, and a two-row `doctors` table for the write-skew demo:
```sh
docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql
```

The experiments are about a *specific interleaving* of statements across two
concurrent transactions, so they need two separate sessions kept open at
the same time rather than a single scripted benchmark. Open two terminals,
each with its own `psql` session:
```sh
docker exec -it lab-postgres psql -U postgres -d labdb   # Session A
docker exec -it lab-postgres psql -U postgres -d labdb   # Session B
```
Each step below gives an ordered sequence of statements labelled `A:`/`B:`
- run them in that exact order, alternating sessions, rather than pasting
a whole block into one session at once.

## Step 1 - non-repeatable reads (prediction 1)

Under READ COMMITTED (the default - no need to set anything):
```
A: BEGIN;
A: SELECT balance FROM accounts WHERE id = 1;        -- 1000
B: UPDATE accounts SET balance = 500 WHERE id = 1;
B: COMMIT;
A: SELECT balance FROM accounts WHERE id = 1;        -- ?
A: COMMIT;
```
What to check: A's second read reflects B's committed change.

Re-seed (`docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql`),
then repeat with `A: BEGIN ISOLATION LEVEL REPEATABLE READ;` instead of
plain `BEGIN`. What to check: A's second read now matches its first,
despite B's commit landing in between.

## Step 2 - lost update: silent vs. loud (prediction 2)

Re-seed. Under READ COMMITTED:
```
A: BEGIN;
B: BEGIN;
A: SELECT balance, version FROM accounts WHERE id = 1;   -- balance=1000, version=0
B: SELECT balance, version FROM accounts WHERE id = 1;   -- balance=1000, version=0
B: UPDATE accounts SET balance = balance - 100, version = version + 1
   WHERE id = 1 AND version = 0;
B: COMMIT;
A: UPDATE accounts SET balance = balance - 100, version = version + 1
   WHERE id = 1 AND version = 0;                          -- ? rows affected
A: COMMIT;
```
What to check: A's `UPDATE` reports `UPDATE 0` - no error, silently
nothing happens. Under load, this is exactly the kind of gap that shows up
between transactions processed and actual balance change - a lost update
that looks like success.

Re-seed, then repeat with both sessions starting
`BEGIN ISOLATION LEVEL REPEATABLE READ;` instead of `BEGIN;`. What to
check: A's final `UPDATE` now raises
`ERROR: could not serialize access due to concurrent update` instead of
silently affecting 0 rows.

## Step 3 - write skew (prediction 3)

Re-seed. Both sessions `BEGIN ISOLATION LEVEL REPEATABLE READ;`:
```
A: BEGIN ISOLATION LEVEL REPEATABLE READ;
B: BEGIN ISOLATION LEVEL REPEATABLE READ;
A: SELECT count(*) FROM doctors WHERE on_call AND id != 1;  -- 1 (Bob)
B: SELECT count(*) FROM doctors WHERE on_call AND id != 2;  -- 1 (Alice)
A: UPDATE doctors SET on_call = false WHERE id = 1;
B: UPDATE doctors SET on_call = false WHERE id = 2;
A: COMMIT;
B: COMMIT;
```
What to check: both commits succeed. `SELECT * FROM doctors;` afterward
shows nobody on call - the invariant is broken, even though each
transaction, in isolation, only ever took itself off call while at least
one *other* doctor appeared to be covering.

Re-seed, then repeat with `BEGIN ISOLATION LEVEL SERIALIZABLE;` on both
sessions. What to check: one of the two `COMMIT`s now fails with a
serialization error; the invariant survives.

## Step 4 (stretch) - quantifying the retry cost (prediction 4)

Drive concurrent load against the single `accounts` row with a small
`pgbench` script, e.g. `race.sql`:
```sql
SELECT version FROM accounts WHERE id = 1 \gset
UPDATE accounts SET balance = balance - 1, version = version + 1
WHERE id = 1 AND version = :version;
```
Run it with many concurrent clients, once per isolation level, re-seeding
and switching the database's default isolation level between runs:
```sh
docker cp race.sql lab-postgres:/tmp/race.sql

docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql
docker exec lab-postgres psql -U postgres -d labdb -c \
  "ALTER DATABASE labdb SET default_transaction_isolation = 'read committed';"
docker exec lab-postgres pgbench -n -f /tmp/race.sql -c 20 -j 4 -T 20 -U postgres labdb

docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql
docker exec lab-postgres psql -U postgres -d labdb -c \
  "ALTER DATABASE labdb SET default_transaction_isolation = 'repeatable read';"
docker exec lab-postgres pgbench -n -f /tmp/race.sql -c 20 -j 4 -T 20 -U postgres labdb
```
What to check: `pgbench`'s `number of failed transactions` should go from
~0 (READ COMMITTED - failures don't exist, updates just silently no-op) to
a real, nonzero count under REPEATABLE READ. Compare *effective* throughput
(successful transactions per second) between the two, not just raw `tps`.

## Tear down

```sh
docker compose -f compose.yml down -v
```
