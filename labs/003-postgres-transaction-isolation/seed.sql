-- Lab 003 seed data.

-- Same shape as lab 002's hot_accounts, for the non-repeatable-read and
-- lost-update demos. A single row is enough - these demos are about a
-- specific two-session interleaving, not throughput under load.
DROP TABLE IF EXISTS accounts;
CREATE TABLE accounts (
    id bigint PRIMARY KEY,
    balance bigint NOT NULL,
    version bigint NOT NULL DEFAULT 0
);
INSERT INTO accounts (id, balance) VALUES (1, 1000);

-- For the write-skew demo: an on-call rotation with the invariant "at
-- least one doctor must be on call at all times". Two transactions can
-- each independently check that invariant, both see it satisfied, and
-- both take themselves off call - violating an invariant neither of them
-- would have violated alone.
DROP TABLE IF EXISTS doctors;
CREATE TABLE doctors (
    id bigint PRIMARY KEY,
    name text NOT NULL,
    on_call boolean NOT NULL
);
INSERT INTO doctors (id, name, on_call) VALUES
    (1, 'Alice', true),
    (2, 'Bob', true);
