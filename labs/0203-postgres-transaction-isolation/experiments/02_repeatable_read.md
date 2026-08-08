## Repeatable Read isolation

With the data freshly seeded:
```sh
docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql
```

Let's check the behavior of the following sequence under READ COMMITTED:
```sh
A: BEGIN;
B: BEGIN;
A: SELECT balance, version FROM accounts WHERE id = 1;   -- balance=1000, version=0
B: SELECT balance, version FROM accounts WHERE id = 1;   -- balance=1000, version=0
B: UPDATE accounts SET balance = balance - 100, version = version + 1
   WHERE id = 1 AND version = 0;
B: COMMIT;
A: UPDATE accounts SET balance = balance - 100, version = version + 1
   WHERE id = 1 AND version = 0;                          -- 0 rows affected
A: COMMIT;
```

This is a typical optimistic locking scenario where A's update was unsuccessful. The transactions are both successful, though. We just have to check that A's update did not modify any rows.

Now let's redo the scenario with REPEATABLE READ isolation:
```sh
A: BEGIN ISOLATION LEVEL REPEATABLE READ;
B: BEGIN ISOLATION LEVEL REPEATABLE READ;
A: SELECT balance, version FROM accounts WHERE id = 1;   -- balance=1000, version=0
B: SELECT balance, version FROM accounts WHERE id = 1;   -- balance=1000, version=0
B: UPDATE accounts SET balance = balance - 100, version = version + 1
   WHERE id = 1 AND version = 0;
B: COMMIT;
A: UPDATE accounts SET balance = balance - 100, version = version + 1
   WHERE id = 1 AND version = 0;                          -- error: could not serialize access due to concurrent update
A: COMMIT;                                                -- ROLLBACK
```

A's update now no longer succeeds, it actually fails with an error, leading to a rollback of the transaction.
