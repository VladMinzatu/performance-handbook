-- pgbench custom script: pessimistic locking via SELECT ... FOR UPDATE.
-- Holds a real row lock across a simulated "think time" (standing in for
-- app-side work between reading and writing), so concurrent attempts on
-- the same row queue up rather than race.
--
-- Variables (set via `pgbench -D name=value`):
--   range      contention level - `range=1` means every client fights over
--              one row, `range=1000` spreads across the whole table.
--   think_time seconds of simulated app work between the read and the
--              write, while the row lock is held.
\set id random(1, :range)
BEGIN;
SELECT balance FROM hot_accounts WHERE id = :id FOR UPDATE;
SELECT pg_sleep(:think_time);
UPDATE hot_accounts SET balance = balance - 1 WHERE id = :id;
COMMIT;
