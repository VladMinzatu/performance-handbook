## Detaction

### From inside the db

First, reset the stats:
```
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT pg_stat_statements_reset();"
```

Next, we can run both versions of the query:
```
docker exec -i lab-postgres psql -U postgres -d labdb -o /dev/null \
  -f /tmp/n_plus_one.sql

docker exec lab-postgres psql -U postgres -d labdb -o /dev/null -c \
  "SELECT * FROM books WHERE author_id = ANY(ARRAY[$IDS]);"
```

Next, check the stats:
```
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
