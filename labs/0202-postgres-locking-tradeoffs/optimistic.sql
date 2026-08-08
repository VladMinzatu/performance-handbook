-- pgbench custom script: optimistic locking via a version column.
-- No row lock is held - the read happens outside any lock, and the final
-- UPDATE only succeeds if `version` hasn't moved since the read. A
-- conflicting concurrent update makes this UPDATE affect 0 rows silently
-- (no error raised) - real applications must check the affected row count
-- and retry. This script deliberately does NOT retry, so lost updates show
-- up directly as a gap between transactions attempted and the actual
-- change in total balance.
--
-- Same `range`/`think_time` variables as pessimistic.sql, via `pgbench -D`.
\set id random(1, :range)
SELECT balance, version FROM hot_accounts WHERE id = :id \gset
SELECT pg_sleep(:think_time);
UPDATE hot_accounts SET balance = balance - 1, version = version + 1
WHERE id = :id AND version = :version;
