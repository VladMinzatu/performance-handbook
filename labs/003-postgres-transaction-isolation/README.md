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
## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down -v
```
