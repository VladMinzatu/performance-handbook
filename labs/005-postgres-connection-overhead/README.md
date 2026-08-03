# 005 - Postgres connection overhead

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

## Hypotheses

**Prediction 1 - connection setup is a real, fixed cost that dominates
throughput for connect-per-operation workloads, even for trivial
queries.** `pgbench`'s `-C` mode (open a fresh connection for every
transaction) running nothing but `SELECT 1` should show dramatically
lower `tps` than the same script run over persistent, reused connections
(`pgbench`'s default) - despite the SQL work being identical and
negligible in both cases. If the gap is large even for a query this
trivial, it's entirely attributable to connection overhead (TCP/socket
setup, auth, backend fork), not anything happening inside Postgres's
executor. Note this needs a throughput-based comparison to show up at
all: timing a single query with `psql`'s `\timing` on a fresh connection
looks no different from timing one on an already-open session, because
`\timing` only measures the query's own round trip *after* the connection
already exists - it structurally can't see connection setup, no matter
how it's invoked.

**Prediction 2 - the mechanism behind that cost is directly observable,
and quantifiable, at the OS level.** Postgres forks one backend process
per connection, and that should show up plainly in kernel-level tracing:
a `sched_process_fork` count that tracks one-per-client for persistent
connections but thousands-per-second for connect-per-transaction traffic;
a real fork-to-ready latency (timestamping the fork and pairing it with
the return of the backend's own initialization routine) that should land
in the same ballpark as `pgbench`'s self-reported connection time,
confirming the two independent measurements agree; and, without needing
any Postgres symbols or internals at all, a plain `accept()` syscall rate
on the postmaster's listening socket that shows the same pattern - a
signal that would work purely as a "something's wrong here" alarm,
without knowing in advance that connection churn is the cause.

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

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down -v
```
