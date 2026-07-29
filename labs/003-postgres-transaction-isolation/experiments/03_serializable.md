## Serializable isolation

With the data freshly seeded:
```
docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql
```

and let's run the following sequence:
```
A: BEGIN ISOLATION LEVEL REPEATABLE READ;
B: BEGIN ISOLATION LEVEL REPEATABLE READ;
A: SELECT count(*) FROM doctors WHERE on_call AND id != 1;  -- 1 (Bob)
B: SELECT count(*) FROM doctors WHERE on_call AND id != 2;  -- 1 (Alice)
A: UPDATE doctors SET on_call = false WHERE id = 1;
B: UPDATE doctors SET on_call = false WHERE id = 2;
A: COMMIT;
B: COMMIT;
```

Although each transaction attempted to set one of the doctors off call while first checking that someone else was on call, we left our database in the undersirable state that nobody is on call. The isolation level in combination with our transaction logic did not work as intended.

But we can achieve the desired behavior by using the SERIALIZABLE isolation level. First reseed:
```
docker exec -i lab-postgres psql -U postgres -d labdb < seed.sql
```

then:
```
A: BEGIN ISOLATION LEVEL SERIALIZABLE;
B: BEGIN ISOLATION LEVEL SERIALIZABLE;
A: SELECT count(*) FROM doctors WHERE on_call AND id != 1;  -- 1 (Bob)
B: SELECT count(*) FROM doctors WHERE on_call AND id != 2;  -- 1 (Alice)
A: UPDATE doctors SET on_call = false WHERE id = 1;
B: UPDATE doctors SET on_call = false WHERE id = 2;
A: COMMIT;
B: COMMIT;                                                  -- ERROR: could not serialize access due to read/write dependencies among transactions
```

And the integrity of the data is saved: Bob is still on call.
