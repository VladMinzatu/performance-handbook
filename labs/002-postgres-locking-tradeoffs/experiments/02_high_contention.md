## High Contention

Re-seed and run the high contention experiment using pessimistic locking (range parameter is 1 now instead of 1000):
```
docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql

docker exec lab-postgres pgbench -D range=1 -D think_time=0.005 \
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
number of transactions actually processed: 2038
number of failed transactions: 0 (0.000%)
latency average = 198.170 ms
initial connection time = 5.140 ms
tps = 100.923689 (without initial connection time)
```

And the actual decrement:
```
 docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT 1000 * 1000000 - SUM(balance) AS actual_decrement FROM hot_accounts;"
 actual_decrement 
------------------
             2038
(1 row)
```

And running the high contention experiment with optimistic locking:
```
docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql

docker exec lab-postgres pgbench -D range=1 -D think_time=0.005 \
  -f /tmp/optimistic.sql -c 20 -j 4 -T 20 -U postgres labdb
```

producing the output:
```
transaction type: /tmp/optimistic.sql
scaling factor: 1
query mode: simple
number of clients: 20
number of threads: 4
maximum number of tries: 1
duration: 20 s
number of transactions actually processed: 57711
number of failed transactions: 0 (0.000%)
latency average = 6.935 ms
initial connection time = 4.765 ms
tps = 2884.058653 (without initial connection time)
```

And the actual decrement:
```
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT 1000 * 1000000 - SUM(balance) AS actual_decrement FROM hot_accounts;"
 actual_decrement 
------------------
             2904
(1 row)
```

As expected, the optimistic locking version maintains the throughput and latency, while the actual decrement is much lower than the low contention scenario -> lots of failed transactions.

The pessimistic locking version, on the other hand, has much lower throughput and much higher latency. As in the low contention scenario, there are no failed transactions (the actual_decrement corresponds to the number of transactions processed), but the contention makes it so we processed a much smaller number of transactions in total.
