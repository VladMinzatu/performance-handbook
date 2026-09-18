-- Lab 0206 seed data.
-- ~3M padded rows, sized to comfortably exceed shared_buffers (64MB, set
-- in compose.yml) - a scan has to actually reach disk, not just
-- Postgres's own cache, for io_method to have anything to do.
-- `bucket` is low-cardinality (20 values) so a bitmap heap scan over a
-- handful of buckets still has to fetch a large, scattered fraction of
-- the table's pages - the access pattern io_method's read overlap is
-- meant to help with.

DROP TABLE IF EXISTS events;

CREATE TABLE events (
    id bigserial PRIMARY KEY,
    bucket integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    payload text NOT NULL
);

INSERT INTO events (bucket, created_at, payload)
SELECT
    (random() * 19)::int,
    now() - (random() * interval '365 days'),
    repeat('x', 200)
FROM generate_series(1, 3000000);

CREATE INDEX events_bucket_idx ON events (bucket);

ANALYZE events;
