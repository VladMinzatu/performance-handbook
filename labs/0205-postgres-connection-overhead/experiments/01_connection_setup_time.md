## Connection setup time

Let's run `pgbench` with a persistent connection:
```sh
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
```sh
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

The effect on the average latency and throughput are clear. But what matters is the constant overhead: the size of this gap is workload-dependent in a specific way: connection setup is a fixed additive cost, not a multiplier. With SELECT 1 (~free), that fixed cost is nearly the entire transaction, which is why the gap is so dramatic (~175x). If the query itself took, say, 50ms, the same fixed connection tax would shrink to a modest percentage overhead rather than a 175x cliff.

Note: Without `-C`, each of pgbench's -c clients opens one connection at the start and reuses it for every transaction over the whole -T duration — connection cost is paid once and amortized across potentially tens of thousands of transactions, which is why it barely shows up in the persistent run's tps. With -C, every single transaction gets its own fresh connect-auth-fork-query-disconnect cycle — it's pgbench's purpose-built way to simulate the "no pooling, no reuse" antipattern.

Note 2: The concurrency (10) is comfortably under `max_connections` (30), so this isolates
connection-churn cost by itself, with no contention effects mixed in.
