## Read Committed isolation

With the data freshly seeded:
```sh
docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql
```

We are checking the behavior of the READ COMMITTED (default) isolation level. With our two sessions A and B (set up as described in the setup steps) we run the following sequence of statements:
```sh
A: BEGIN;
A: SELECT balance FROM accounts WHERE id = 1;        -- 1000
B: UPDATE accounts SET balance = 500 WHERE id = 1;
A: SELECT balance FROM accounts WHERE id = 1;        -- 500
A: COMMIT;
```

A's second read return 500, reflecting the update from B. As expected, A sees the committed change by B during its transaction.

Note that B's statement autocommits (single statement). If B were still in the middle of a transaction when A's second statement ran, then B's change would not be visible to A even under this isolation level.

```sh
A: BEGIN;
A: SELECT balance FROM accounts WHERE id = 1;        -- 1000
B: BEGIN;
B: UPDATE accounts SET balance = 500 WHERE id = 1;
A: SELECT balance FROM accounts WHERE id = 1;        -- 1000
B: COMMIT;
A: SELECT balance FROM accounts WHERE id = 1;        -- 500
A: COMMIT;
```

The most useful implication of READ COMMITTED: because each statement re-evaluates against the latest committed data, a plain `UPDATE accounts SET balance = balance - 1 WHERE id = 1` is completely race-safe on its own — no version column, no SELECT ... FOR UPDATE needed. Postgres re-checks the row and computes balance - 1 against whatever the current committed value is at the moment it actually applies the write, not a value read earlier.