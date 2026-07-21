## Index Test

Let's add the index to the previously set up DB:
```
docker exec lab-postgres psql -U postgres -d labdb -c \
  "CREATE INDEX CONCURRENTLY idx_orders_customer_id ON orders (customer_id);"
```

And let's run our test query again:
```
docker exec lab-postgres psql -U postgres -d labdb -c \
  "EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM orders WHERE customer_id = $CID;"
                                                           QUERY PLAN                                                           
--------------------------------------------------------------------------------------------------------------------------------
 Bitmap Heap Scan on orders  (cost=4.52..48.13 rows=11 width=33) (actual time=0.068..0.187 rows=9 loops=1)
   Recheck Cond: (customer_id = 313952)
   Heap Blocks: exact=9
   Buffers: shared hit=3 read=9
   ->  Bitmap Index Scan on idx_orders_customer_id  (cost=0.00..4.51 rows=11 width=0) (actual time=0.040..0.040 rows=9 loops=1)
         Index Cond: (customer_id = 313952)
         Buffers: shared hit=3
 Planning:
   Buffers: shared hit=87 read=4
 Planning Time: 3.440 ms
 Execution Time: 0.245 ms
(11 rows)
```

The output shows the enormous difference in run time (from 87.5ms to 0.25ms), along with the vastly reduced number of pages read (12 vs ~43k).

We can also see the reference to our index: `idx_orders_customer_id`.

And for an extra confirmation that the index was used:
```
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT idx_scan, idx_tup_read FROM pg_stat_user_indexes WHERE indexrelname = 'idx_orders_customer_id';"
 idx_scan | idx_tup_read 
----------+--------------
        1 |            9
(1 row)
```
