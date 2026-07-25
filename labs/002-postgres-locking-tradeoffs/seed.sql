-- Lab 002 seed data.
-- A small table of "accounts" to contend over, modeling a common
-- real-world pattern: many concurrent transactions trying to update the
-- same handful of rows (an inventory count, an account balance, a booking
-- slot). `version` backs the optimistic-locking test.
--
-- Re-run this before each measurement to reset balances/versions to a
-- known baseline, so "expected vs actual decrement" is clean per run.

DROP TABLE IF EXISTS hot_accounts;

CREATE TABLE hot_accounts (
    id bigint PRIMARY KEY,
    balance bigint NOT NULL,
    version bigint NOT NULL DEFAULT 0
);

INSERT INTO hot_accounts (id, balance)
SELECT id, 1000000
FROM generate_series(1, 1000) AS id;
