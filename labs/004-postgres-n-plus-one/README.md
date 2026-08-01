# 004 - Postgres N+1 queries vs. batching

Uses the shared lab infrastructure in [tools/](../tools/README.md). The
`analysis` container is only needed for setting up the network and the optional stretch step
(injecting realistic network latency) - the core lab is plain `psql` and
shell timing.

## Background

The N+1 query problem is one of the most common real-world application
performance bugs: fetch a list of N parent rows, then - typically via an
ORM's lazy-loaded relationship, one call site at a time, without anyone
deciding to do it this way on purpose - issue one additional query *per
parent* to fetch its related child rows. N+1 round trips instead of 2.

It's easy to miss because each individual query can be completely healthy
on its own - an indexed lookup returning a handful of rows in well under a
millisecond of database-side work. The part that doesn't show up in a
per-query `EXPLAIN ANALYZE` is the *round trip* itself: the network
hop plus protocol overhead of sending a query and waiting for its
response, paid once per query rather than once per batch. That cost is
easy to overlook precisely because it's close to invisible on a
developer's laptop talking to `localhost` - and then very much not
invisible once the same code talks to a database over a real network in
production.

The two standard fixes both replace N round trips with one:
- `WHERE id = ANY($1)` - fetch all the needed child rows in a single query
  against an array of parent ids, then group the results by parent id in
  the application.
- A `JOIN` - fetch parents and children together in one query, one row per
  (parent, child) pair.

## Hypotheses

**Prediction 1 - batching wins, and realistic network latency makes the
gap dramatic.** With a small amount of round-trip latency injected between
client and database (standing in for a real network, rather than the
near-zero latency of talking to `localhost`), fetching the same books for
the same 100 authors as 100 individual `SELECT ... WHERE author_id = $1`
round trips should cost far more wall-clock time than fetching them as one
`SELECT ... WHERE author_id = ANY(...)` round trip - multiplied by roughly
N x the injected latency, since every one of the N round trips pays it,
not just one. Without any injected latency the same comparison should
still favor batching (each round trip carries some fixed protocol/context-
switch overhead regardless of network delay), but that gap is expected to
be far less dramatic - this lab's setup always runs with latency injected,
so that plain-localhost case is an inference from the mechanism, not
something separately measured here.

**Prediction 2 - the gap scales with N, in different ways for each
approach.** Repeating Prediction 1's comparison at increasing N (e.g. 10,
100, 1000) should show the N+1 approach's wall-clock time growing
roughly linearly with N (each additional parent adds one more fixed-cost,
latency-multiplied round trip), while the batched approach's time stays
close to flat (still one round trip, just a bigger result set).

**Prediction 3 - N+1 has a distinctive, detectable signature at two
different layers, without needing to read application code.** Inside the
database, `pg_stat_statements` normalizes literal values out of query
text, so all N individual `author_id = $i` lookups collapse into a
*single* tracked entry with an outsized `calls` count - directly visible
without touching the app. Entirely outside the database, attaching to the
client library's query-send function (a uprobe on `libpq`'s
`PQsendQuery`) surfaces the same repeated-query-shape signature *plus*
exact timing - a tight burst of near-identical calls with no gap between
them for application "think time" - without any visibility into the
database or the application's source at all.

**Prediction 4 - a detected pattern can be confirmed as genuine
round-trip overhead, not just repeated calls, by counting the actual
network syscalls involved.** Once Prediction 4 has flagged a suspicious
query pattern, attaching `strace` to the already-running client process
and counting `sendto`/`recvfrom` calls should show a 1:1 ratio between
"number of times this query was called" and "number of separate network
round trips" - proving each call really did hit the network on its own,
rather than being pipelined or coalesced by the client library, and
turning "this looks suspicious" into a hard, measured number of wasted
round trips.

## Setup

Start Postgres for this lab:
```sh
docker compose -f compose.yml up -d
```

Load the seed data (1000 authors, 10 books each, `books.author_id`
indexed):
```sh
docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql
```

Enable `pg_stat_statements` (used in Step 5) - it's preloaded via
`compose.yml`, so this just needs to register the SQL-level views:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
```

Both query shapes below are generated as plain SQL text (a shell loop for
the N+1 case, a single line for the batched case) and timed with the
shell's `time`, running against `psql` with output discarded
(`-o /dev/null`) so we're measuring query round trips, not terminal
rendering:
```sh
N=100

# N individual queries, e.g. N=100:
for i in $(seq 1 $N); do echo "SELECT * FROM books WHERE author_id = $i;"; done \
  > /tmp/n_plus_one.sql
docker cp /tmp/n_plus_one.sql lab-postgres:/tmp/n_plus_one.sql

# one batched query for the same 100 ids:
# (paste, not `seq -s,`, so this doesn't leave a trailing comma - BSD/macOS
# `seq -s` appends the separator after the last number too, unlike GNU seq)
IDS=$(seq 1 $N | paste -sd, -)
```

Next, we need to set up realistic network delay (note that we are targeting the 
Postgres process and the loopback iterface):
```
PG_PID=$(docker inspect -f '{{.State.Pid}}' lab-postgres)

docker compose -f ../tools/analysis/compose.yml exec -T analysis \
  nsenter --net=/proc/$PG_PID/ns/net -- tc qdisc add dev lo root netem delay 5ms
```

For this to be effective, we will run the experiments by forcing connecting over TCP,
not Unix socket by adding -h 127.0.0.1, which will use the loopback interface in our setup, e.g:
```
docker exec -e PGPASSWORD=postgres lab-postgres psql -h 127.0.0.1 -U postgres -d labdb \
  -o /dev/null -f /tmp/n_plus_one.sql
``` 

**Note**: The `eth0` is the right interface if the client runs outside this container —  e.g. from the analysis container or the host, hitting the exposed 5432 port over the real labnet bridge network. That's arguably a more realistic setup But since we're running the client via docker exec into the same container, the lo + -h 127.0.0.1 setup is the quick path.

To remove the injected latency afterward:
```sh
docker compose -f ../tools/analysis/compose.yml exec -T analysis \
  nsenter --net=/proc/$PG_PID/ns/net -- tc qdisc del dev lo root netem
```

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down -v
```
