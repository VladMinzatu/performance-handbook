## Connection setup time

Let's run `pgbench` with a persistent connection:
```
# persistent connections (pgbench's default - connect once, reuse)
docker exec lab-postgres pgbench -n -c 10 -j 4 -T 10 -f /tmp/select1.sql -U postgres labdb

pgbench (16.14 (Debian 16.14-1.pgdg13+1))
transaction type: /tmp/select1.sql
scaling factor: 1
query mode: simple
number of clients: 10
number of threads: 4
maximum number of tries: 1
duration: 10 s
number of transactions actually processed: 4477970
number of failed transactions: 0 (0.000%)
latency average = 0.022 ms
initial connection time = 3.428 ms
tps = 447883.979069 (without initial connection time)
```


Then rerun with one connection per transaction:
```
# connect-per-transaction
docker exec lab-postgres pgbench -n -C -c 10 -j 4 -T 10 -f /tmp/select1.sql -U postgres labdb

pgbench (16.14 (Debian 16.14-1.pgdg13+1))
transaction type: /tmp/select1.sql
scaling factor: 1
query mode: simple
number of clients: 10
number of threads: 4
maximum number of tries: 1
duration: 10 s
number of transactions actually processed: 33695
number of failed transactions: 0 (0.000%)
latency average = 2.968 ms
average connection time = 1.180 ms
tps = 3369.004756 (including reconnection times)
```

The effect on the average latency and throughput are clear: orders of magnitude difference!