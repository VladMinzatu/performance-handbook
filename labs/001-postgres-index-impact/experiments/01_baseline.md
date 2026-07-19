## Baseline

Following the setup steps in the `../README.md`, we will use the following customer id from the db for testing:
```
CID=313952
```

Running a query for our specific customer id produces the following output:
```
 $ docker exec lab-postgres psql -U postgres -d labdb -c \
  "EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM orders WHERE customer_id = $CID;"
                                                      QUERY PLAN                                                      
----------------------------------------------------------------------------------------------------------------------
 Gather  (cost=1000.00..70046.77 rows=11 width=33) (actual time=17.348..87.467 rows=9 loops=1)
   Workers Planned: 2
   Workers Launched: 2
   Buffers: shared hit=12334 read=30670
   ->  Parallel Seq Scan on orders  (cost=0.00..69045.67 rows=5 width=33) (actual time=13.506..84.773 rows=3 loops=3)
         Filter: (customer_id = 313952)
         Rows Removed by Filter: 1666664
         Buffers: shared hit=12334 read=30670
 Planning:
   Buffers: shared hit=69
 Planning Time: 0.204 ms
 Execution Time: 87.534 ms
(12 rows)

```

Main points to note here:
- As expected, parallel sequential scan is used in the absence of an index.
- The time to first row of 17.3 ms and the total time of 87.5 ms to return the 9 rows
- Pages found in Postgres's own shared buffers cache: 12334, so no read OS call necessary. For 30670 pages, Postgres had to ask the OS for them, though this doesn't necessarily imply disk I/0 if the OS had them cached.
- That's a total of ~43k * 8KB pages ≈ 336MB touched — confirming the whole table read to return 9 rows.

And for an extra confirmation of no index used:
```
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT seq_scan, seq_tup_read, idx_scan FROM pg_stat_user_tables WHERE relname = 'orders';"
 seq_scan | seq_tup_read | idx_scan 
----------+--------------+----------
        5 |     10000000 |        0
```