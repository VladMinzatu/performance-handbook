## Confirmation

Detection (`02_detection.md`) told us *which* query shape is suspicious and
roughly *when* it fires - a `calls` count in `pg_stat_statements`, or a
burst of near-identical queries with no gap between them from the `libpq`
uprobe. Neither of those actually proves that each call hit the network as
its own separate round trip, rather than, say, `libpq` pipelining them
together under the hood. Confirmation closes that gap: attach `strace` to
the running client process and count the actual network syscalls.

### Via strace, attached to the running client process

Give the traced script a moment's head start so there's time to attach
before the real work happens:
```
(echo "SELECT pg_sleep(0.5);"; cat /tmp/n_plus_one.sql) > /tmp/n_plus_one_delayed.sql
docker cp /tmp/n_plus_one_delayed.sql lab-postgres:/tmp/n_plus_one_delayed.sql

docker exec lab-postgres psql -U postgres -d labdb -o /dev/null -f /tmp/n_plus_one_delayed.sql &

sleep 0.2
PSQL_PID=$(docker top lab-postgres -eo pid,comm | awk '/psql/ {print $1}')
docker compose -f ../tools/analysis/compose.yml exec -T analysis \
  strace -c -e trace=network -p "$PSQL_PID"
wait
```
(`docker top` reports host-visible PIDs directly, so `strace` running in
the `analysis` container can attach to it with no PID-namespace
translation needed.)

producing:
```
strace: Process 16501 attached
% time     seconds  usecs/call     calls    errors syscall
------ ----------- ----------- --------- --------- ----------------
 59.35    0.000276           1       202       101 recvfrom
 40.65    0.000189           1       101           sendto
------ ----------- ----------- --------- --------- ----------------
100.00    0.000465           1       303       101 total
```

Same recipe, batched query instead (`SELECT pg_sleep(0.5);` followed by
`SELECT * FROM books WHERE author_id = ANY(ARRAY[$IDS]);` in the delayed
script):
```
strace: Process 16552 attached
% time     seconds  usecs/call     calls    errors syscall
------ ----------- ----------- --------- --------- ----------------
 91.30    0.000021           3         6         2 recvfrom
  8.70    0.000002           1         2           sendto
------ ----------- ----------- --------- --------- ----------------
100.00    0.000023           2         8         2 total
```

`sendto` is the cleanest confirmation available: 101 calls for the N+1
script (one `pg_sleep` warm-up plus 100 individual queries) versus 2 for
the batched script (the same warm-up plus one query) - a 1:1 match with
the number of statements actually sent, proving each one really did leave
as its own separate write to the socket rather than being coalesced by
`libpq`. `recvfrom` runs a bit higher than `sendto` in both cases (202 vs
101, and 6 vs 2) because of the `errors` column: `libpq` polls the socket
non-blockingly, so a chunk of those calls are `EAGAIN` ("not ready yet")
before the real read succeeds - accounting for exactly 101 and 2
respectively, matching `sendto`'s count 1:1 once the retries are
subtracted out. Either way, the ratio between the two runs (~50x more
network syscalls for the same 100 logical rows of interest) is the number
that turns "this looks suspicious" into "this is measurably costing N
round trips instead of 1."
