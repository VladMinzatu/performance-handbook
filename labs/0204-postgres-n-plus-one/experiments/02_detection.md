## Detaction

### From inside the db

First, reset the stats:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT pg_stat_statements_reset();"
```

Next, we can run both versions of the query:
```sh
docker exec -i lab-postgres psql -U postgres -d labdb -o /dev/null \
  -f /tmp/n_plus_one.sql

docker exec lab-postgres psql -U postgres -d labdb -o /dev/null -c \
  "SELECT * FROM books WHERE author_id = ANY(ARRAY[$IDS]);"
```

Next, check the stats:
```sh
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT query, calls, mean_exec_time, rows
   FROM pg_stat_statements
   WHERE query ILIKE '%books%'
   ORDER BY calls DESC;"
                                                                                                                                                                                                                           query                                                                                                                                                                                                                           | calls |    mean_exec_time    | rows 
-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+-------+----------------------+------
 SELECT * FROM books WHERE author_id = $1                                                                                                                                                                                                                                                                                                                                                                                                                  |   100 | 0.008192160000000002 | 1000
 SELECT * FROM books WHERE author_id = ANY(ARRAY[$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37,$38,$39,$40,$41,$42,$43,$44,$45,$46,$47,$48,$49,$50,$51,$52,$53,$54,$55,$56,$57,$58,$59,$60,$61,$62,$63,$64,$65,$66,$67,$68,$69,$70,$71,$72,$73,$74,$75,$76,$77,$78,$79,$80,$81,$82,$83,$84,$85,$86,$87,$88,$89,$90,$91,$92,$93,$94,$95,$96,$97,$98,$99,$100]) |     1 |             0.309421 | 1000
(2 rows)

```

The 100 individual lookups, despite each having a *different* literal `author_id`, all collapse into a **single row** - `SELECT * FROM books WHERE author_id = $1` - because Postgres normalizes out literal constants before tracking. We can see 100 such calls with a mean exec time of 0.0081s (so in total, 0.81s), vs one query with an exec time of just 0.3s.

This is the signal a real production investigation would look for: one normalized query with a suspiciously large `calls` count (often a multiple of some other query's `calls` - "called once per row of whatever that other query returned" is the N+1 tell), found without looking at a single line of application code.

### From outside, entirely - via a uprobe on the client library

`pg_stat_statements` still needs database access. We can get the same signal - and more of it - from completely outside the app and the database, by attaching to `PQsendQuery`, the `libpq` function `psql` calls to issue each query. It's a public, exported function of a shared library, so it's always present in the dynamic symbol table, unlike arbitrary internal functions that may or may not survive stripping.

First, find the library and the Postgres container's PID (needed to reach its filesystem from the `analysis` container):
```sh
docker exec lab-postgres bash -c "find / -name 'libpq.so*' 2>/dev/null"
/usr/lib/aarch64-linux-gnu/libpq.so.5
/usr/lib/aarch64-linux-gnu/libpq.so.5.18

PG_PID=$(docker inspect -f '{{.State.Pid}}' lab-postgres)
```

Then attach the uprobe from the `analysis` container, printing a timestamp and the actual query text for every call:
```sh
docker compose -f ../tools/analysis/compose.yml exec -T analysis bash -c "
  bpftrace -e '
    uprobe:/proc/${PG_PID}/root/usr/lib/aarch64-linux-gnu/libpq.so.5:PQsendQuery
    {
      printf(\"%s pid=%d query=%s\n\", strftime(\"%H:%M:%S.%f\", nsecs), pid, str(arg1));
    }
    interval:s:6 { exit(); }
  '
"
```

Trigger the N+1 script while that's running:
```sh
docker exec lab-postgres psql -U postgres -d labdb -o /dev/null -f /tmp/n_plus_one.sql
```

producing (100 lines total, first few and last shown):
```sh
08:43:50.513026 pid=17012 query=SELECT * FROM books WHERE author_id = 1;
08:43:50.514093 pid=17012 query=SELECT * FROM books WHERE author_id = 2;
08:43:50.514171 pid=17012 query=SELECT * FROM books WHERE author_id = 3;
08:43:50.514224 pid=17012 query=SELECT * FROM books WHERE author_id = 4;
08:43:50.514272 pid=17012 query=SELECT * FROM books WHERE author_id = 5;
08:43:50.514324 pid=17012 query=SELECT * FROM books WHERE author_id = 6;
...
08:43:50.518964 pid=17012 query=SELECT * FROM books WHERE author_id = 100;
```

This is a stronger signal than a syscall/round-trip count would be: we get the same repeated-query-shape tell as `pg_stat_statements` (varying only in the literal), plus the actual timing - all 100 calls fire from the same pid, back-to-back, roughly 50-90us apart with no gap for application "think time" in between. That tight, uninterrupted cadence *is* the N+1 fingerprint, and unlike the `pg_stat_statements` view, it doesn't require knowing in advance which operation to go compare against something else - the burst pattern is visible on its own, from outside the app and the database entirely.

One limitation worth being upfront about: this only works for clients that actually link `libpq` (`psql`, `psycopg2`, and similar). Drivers that implement the wire protocol themselves instead of linking `libpq` (Go's `pgx`, Node's `pg`) won't have this symbol at all - `pg_stat_statements`, or a wire-level packet capture, remain the fallback there.
