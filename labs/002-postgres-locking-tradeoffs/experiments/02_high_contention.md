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


We can also sample the pg_stat_activity to check the locking behaviour during these runs:
```
docker exec lab-postgres psql -U postgres -d labdb -c \
  "SELECT pid, wait_event_type, wait_event, state, query
   FROM pg_stat_activity
   WHERE datname = 'labdb' AND wait_event_type = 'Lock';"
```

During the pessimistic locking run we get:
```
pid  | wait_event_type |  wait_event   | state  |                           query                           
------+-----------------+---------------+--------+-----------------------------------------------------------
 1986 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 1987 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 1988 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 1989 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 1990 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 1991 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 1992 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 1993 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 1994 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 1995 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 1996 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 1998 | Lock            | transactionid | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 1999 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 2000 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 2001 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 2002 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 2003 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 2004 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
 2005 | Lock            | tuple         | active | SELECT balance FROM hot_accounts WHERE id = 1 FOR UPDATE;
(19 rows)
``` 

We can see the the one backend parked waiting on the current transaction to finish, with another 18 queuing up behind it, contending for the brief per-tuple lock just to register themselves as a waiter. Once the transaction is in, though, it is guaranteed to succeed.

In the optimistic case, we just sometimes see something like this:
```
 pid  | wait_event_type |  wait_event   | state  |                                query                                 
------+-----------------+---------------+--------+----------------------------------------------------------------------
 2036 | Lock            | transactionid | active | UPDATE hot_accounts SET balance = balance - 1, version = version + 1+
      |                 |               |        | WHERE id = 1 AND version = 750;
 2052 | Lock            | tuple         | active | UPDATE hot_accounts SET balance = balance - 1, version = version + 1+
      |                 |               |        | WHERE id = 1 AND version = 750;
```

A lot less contention for sure, as there is no explicit locking in optimistic.sql, but the update does implicitly need a row lock the instant it runs and collisions can happen if two UPDATEs land virtually at the same instant. In that case, one of them wins. But this locking doesn't take the whole `think_time`, as in the pessimistic case, it just takes the few milliseconds needed for the actual write.
