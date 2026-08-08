-- Lab 005 pgbench script: the simplest possible query, deliberately.
-- The point of this lab is connection cost, not query cost - so the query
-- itself should contribute as close to zero as possible, isolating
-- whatever else the numbers show as connection/pooling overhead.
SELECT 1;
