## Low contention

Re-seed and run the low contention experiment using pessimistic locking:
```
docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql

docker exec lab-postgres pgbench -D range=1000 -D think_time=0.005 \
  -f /tmp/pessimistic.sql -c 20 -j 4 -T 20 -U postgres labdb
```

producing the output:
```
transaction type: /tmp/pessimistic.sql
scaling factor: 1
query mode: simple
number of clients: 20
number of threads: 4
maximum number of tries: 1
duration: 20 s
number of transactions actually processed: 51568
number of failed transactions: 0 (0.000%)
latency average = 7.759 ms
initial connection time = 6.265 ms
tps = 2577.777080 (without initial connection time)
```

And the actual decrement:
```
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT 1000 * 1000000 - SUM(balance) AS actual_decrement FROM hot_accounts;"
 actual_decrement 
------------------
            51568
(1 row)
```

And running the low contention experiment with optimistic locking:
```
docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql

docker exec lab-postgres pgbench -D range=1000 -D think_time=0.005 \
  -f /tmp/optimistic.sql -c 20 -j 4 -T 20 -U postgres labdb
```
producing output:
```
transaction type: /tmp/optimistic.sql
scaling factor: 1
query mode: simple
number of clients: 20
number of threads: 4
maximum number of tries: 1
duration: 20 s
number of transactions actually processed: 49549
number of failed transactions: 0 (0.000%)
latency average = 8.076 ms
initial connection time = 7.163 ms
tps = 2476.454713 (without initial connection time)
```

And the actual decrement:
```
 docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT 1000 * 1000000 - SUM(balance) AS actual_decrement FROM hot_accounts;"

 actual_decrement 
------------------
            48764
(1 row)
```

As expected, the throughput and latencies really are comparable.

Also, the actual_decrement is equal exactly the number of transactions for the pessimistic locking scenario (that's what the locking guarantee is for), whereas in the optimistic case, it's a bit lower (the difference corresponding to the number of failed transactions), but only slightly so (due to the low contention in the tested scenario).
