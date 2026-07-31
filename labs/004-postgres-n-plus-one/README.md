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

**Prediction 1 - batching wins even on localhost.** Fetching the same
books for the same 100 authors should take noticeably longer as 100
individual `SELECT ... WHERE author_id = $1` round trips than as one
`SELECT ... WHERE author_id = ANY(...)` round trip, even with negligible
network latency - because each round trip carries fixed overhead
(protocol framing, a context switch, planning) independent of how little
data it returns.

**Prediction 2 - the gap scales with N, in different ways for each
approach.** Repeating Prediction 1's comparison at increasing N (e.g. 10,
100, 1000) should show the N+1 approach's wall-clock time growing
roughly linearly with N (each additional parent adds one more fixed-cost
round trip), while the batched approach's time stays close to flat
(still one round trip, just a bigger result set).

**Prediction 3 - JOIN-based batching moves more bytes than
`ANY()`-based batching.** Both fixes solve the round-trip problem, but a
JOIN repeats every parent column on every one of its child rows. For
authors with 10 books each, a JOIN result duplicates each author's `name`
and `bio` 10 times over; a separate `WHERE author_id = ANY(...)` query
against just the `books` table returns each book's row and nothing else,
with parent data grouped in afterward on the client. The difference in
bytes returned should be measurable and should grow with how many
children each parent has and how wide the parent row is.

**Prediction 4 (stretch) - the whole story gets dramatically worse under
realistic network latency.** Repeating Prediction 1/2 with a few
milliseconds of artificial latency injected between client and database
should show the N+1 approach's cost multiply by roughly N x (added
latency), while the batched approach barely moves (still one round trip).
This is the mechanism behind why N+1 bugs are so easy to ship: they're
nearly free on a local dev database and expensive the moment a real
network sits in between.

**Prediction 5 - N+1 has a distinctive, detectable signature at two
different layers, without needing to read application code.** Inside the
database, `pg_stat_statements` normalizes literal values out of query
text, so all N individual `author_id = $i` lookups collapse into a
*single* tracked entry with an outsized `calls` count - directly visible
without touching the app. Entirely outside the database, counting the
number of socket round trips (`sendto`/`recvfrom` pairs) a client process
makes for one logical operation gives the same signal even with zero
visibility into query text at all: N+1 shows up as N round trips,
batching as 1, regardless of what the queries actually say.

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
# N individual queries, e.g. N=100:
for i in $(seq 1 100); do echo "SELECT * FROM books WHERE author_id = $i;"; done \
  > /tmp/n_plus_one.sql
docker cp /tmp/n_plus_one.sql lab-postgres:/tmp/n_plus_one.sql

# one batched query for the same 100 ids:
IDS=$(seq -s, 1 100)
```

## Step 1 - N+1 vs. batched, same N (prediction 1)

```sh
time docker exec -i lab-postgres psql -U postgres -d labdb -o /dev/null \
  -f /tmp/n_plus_one.sql

time docker exec lab-postgres psql -U postgres -d labdb -o /dev/null -c \
  "SELECT * FROM books WHERE author_id = ANY(ARRAY[$IDS]);"
```
What to check: total wall time for the 100 individual queries vs. the one
batched query. Both return the same rows in total - the difference is
round trips, not data volume or per-query plan cost (check
`EXPLAIN (ANALYZE, BUFFERS)` on one individual lookup if you want to
confirm it's already a cheap index scan on its own).

## Step 2 - scaling with N (prediction 2)

Repeat Step 1's setup and timing at a few values of N (e.g. 10, 100, 1000
- regenerate `/tmp/n_plus_one.sql` and `$IDS` for each). Tabulate wall
time for both approaches against N and look at the shape of the growth,
not just the endpoints.

## Step 3 - JOIN vs. `ANY()` batching (prediction 3)

For the same 100 ids, compare response size between a JOIN and an
`ANY()`-based fetch:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT a.id, a.name, a.bio, b.id, b.title
   FROM authors a JOIN books b ON b.author_id = a.id
   WHERE a.id = ANY(ARRAY[$IDS]);" > /tmp/out_join.txt
wc -c /tmp/out_join.txt

docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT * FROM books WHERE author_id = ANY(ARRAY[$IDS]);" > /tmp/out_any.txt
wc -c /tmp/out_any.txt
```
What to check: `out_join.txt` should be noticeably larger than
`out_any.txt` - the JOIN result repeats each author's `name`/`bio` once
per book, while the `ANY()` result only carries book columns (the
application would already have the author data from the first query, and
groups these results onto it afterward).

## Step 4 (stretch) - under realistic network latency (prediction 4)

Inject artificial latency into `lab-postgres`'s own network interface,
using `nsenter` from the `analysis` container to reach into its network
namespace (no changes to the Postgres image needed):
```sh
docker compose -f ../tools/analysis/compose.yml up -d --build

PG_PID=$(docker inspect -f '{{.State.Pid}}' lab-postgres)
docker compose -f ../tools/analysis/compose.yml exec -T analysis \
  nsenter --net=/proc/$PG_PID/ns/net -- tc qdisc add dev eth0 root netem delay 5ms
```
(assumes the container's primary interface is `eth0`, the default for a
single-network compose service - check with
`docker exec lab-postgres cat /proc/net/dev` if that doesn't hold).

Re-run Step 1 (and optionally Step 2) with this latency in place. What to
check: the batched query's wall time should barely change (still ~1 round
trip plus ~5ms), while the N+1 script's wall time should jump by roughly
`N x 5ms` on top of its baseline - a gap that was maybe barely noticeable
in Step 1 should now be dramatic.

Remove the injected latency afterward:
```sh
docker compose -f ../tools/analysis/compose.yml exec -T analysis \
  nsenter --net=/proc/$PG_PID/ns/net -- tc qdisc del dev eth0 root netem
```

## Step 5 - detecting it from inside the database (prediction 5)

Reset stats, re-run the N+1 script and the batched query from Step 1, then
look at what got tracked:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT pg_stat_statements_reset();"

docker exec -i lab-postgres psql -U postgres -d labdb -o /dev/null \
  -f /tmp/n_plus_one.sql

docker exec lab-postgres psql -U postgres -d labdb -o /dev/null -c \
  "SELECT * FROM books WHERE author_id = ANY(ARRAY[$IDS]);"

docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT query, calls, mean_exec_time, rows
   FROM pg_stat_statements
   WHERE query ILIKE '%books%'
   ORDER BY calls DESC;"
```
What to check: the 100 individual lookups, despite each having a
*different* literal `author_id`, all collapse into a **single row** -
`SELECT * FROM books WHERE author_id = $1` - because Postgres normalizes
out literal constants before tracking. That row's `calls` should read
~100; the `ANY(...)` query's row should read `calls = 1`. This is the
same signal a real production investigation would look for: one
normalized query with a suspiciously large `calls` count (often a
multiple of some other query's `calls` - "called once per row of
whatever that other query returned" is the N+1 tell), found without
looking at a single line of application code.

## Step 6 - confirming it from outside, at the syscall level (prediction 5)

`pg_stat_statements` needs database access and query-text visibility.
This step gets the same answer with neither - by counting the actual
network round trips a client process makes, treating both the app and the
database as a black box.

Give the traced script a moment's head start so there's time to attach
before the real work happens:
```sh
(echo "SELECT pg_sleep(0.5);"; cat /tmp/n_plus_one.sql) > /tmp/n_plus_one_delayed.sql
docker cp /tmp/n_plus_one_delayed.sql lab-postgres:/tmp/n_plus_one_delayed.sql

docker exec lab-postgres psql -U postgres -d labdb -o /dev/null \
  -f /tmp/n_plus_one_delayed.sql &

sleep 0.2
PSQL_PID=$(docker top lab-postgres -eo pid,comm | awk '/psql/ {print $1}')
docker compose -f ../tools/analysis/compose.yml exec -T analysis \
  strace -c -e trace=network -p "$PSQL_PID"
wait
```
(`docker top` reports host-visible PIDs directly, so no PID-namespace
translation is needed to hand `$PSQL_PID` to `strace` running in a
different container.) Repeat the same recipe for the batched query
(prefix its `-c` query with a `SELECT pg_sleep(0.5);` via a small wrapper
script, same idea).

What to check: `strace -c`'s summary table should show roughly 100
`sendto`/`recvfrom` pairs for the N+1 script and ~1 pair for the batched
query - the same N-vs-1 signature as Step 5, but derived purely from
watching sockets, with zero insight into what the queries actually say.

## Tear down

```sh
docker compose -f compose.yml down -v
```
