-- pgbench custom script: steady-state INSERT load against `orders`, used to
-- measure write-side cost before/after the index exists. Run with `pgbench
-- -n -f bench_insert.sql` (no -i - we supply our own table, not pgbench's).
\set customer_id random(1, 500000)
INSERT INTO orders (customer_id, created_at, amount, status)
VALUES (:customer_id, now(), (random() * 1000)::numeric(10, 2), 'pending');
