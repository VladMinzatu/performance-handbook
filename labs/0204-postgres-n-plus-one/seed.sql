-- Lab 004 seed data.
-- A classic parent/child shape: authors and their books. `author_id` is
-- indexed deliberately, so each individual lookup is already cheap on the
-- database side - the point of this lab is round-trip overhead, not query
-- plan cost. `bio` is padded to a few hundred bytes so that JOIN-based
-- batching's parent-column duplication shows up clearly in response size.

DROP TABLE IF EXISTS books;
DROP TABLE IF EXISTS authors;

CREATE TABLE authors (
    id bigint PRIMARY KEY,
    name text NOT NULL,
    bio text NOT NULL
);

CREATE TABLE books (
    id bigint PRIMARY KEY,
    author_id bigint NOT NULL REFERENCES authors (id),
    title text NOT NULL
);

CREATE INDEX idx_books_author_id ON books (author_id);

-- 1000 authors
INSERT INTO authors (id, name, bio)
SELECT id, 'Author ' || id, repeat('x', 500)
FROM generate_series(1, 1000) AS id;

-- 10 books per author -> 10,000 books
INSERT INTO books (id, author_id, title)
SELECT
    row_number() OVER (),
    author_id,
    'Book ' || book_num || ' by author ' || author_id
FROM generate_series(1, 1000) AS author_id
CROSS JOIN generate_series(1, 10) AS book_num;

ANALYZE authors;
ANALYZE books;
