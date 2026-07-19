-- Lab 001 seed data.
-- 5M orders spread over 500k customers (~10 rows/customer on average), so a
-- lookup for a single customer_id is highly selective (~0.0002% of rows) -
-- exactly the case where an index should matter.

DROP TABLE IF EXISTS orders;

CREATE TABLE orders (
    id bigserial PRIMARY KEY,
    customer_id integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    amount numeric(10, 2) NOT NULL,
    status text NOT NULL
);

INSERT INTO orders (customer_id, created_at, amount, status)
SELECT
    (random() * 499999)::int + 1,
    now() - (random() * interval '365 days'),
    (random() * 1000)::numeric(10, 2),
    (ARRAY['pending', 'paid', 'shipped', 'cancelled'])[floor(random() * 4 + 1)]
FROM generate_series(1, 5000000);

ANALYZE orders;
