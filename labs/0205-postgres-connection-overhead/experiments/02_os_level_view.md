## OS Level view

We can re-run the benchmarks from the previous experiment with the following running:
```sh
PG_PID=$(docker inspect -f '{{.State.Pid}}' lab-postgres)
docker compose -f ../tools/analysis/compose.yml exec -T analysis bash -c "
  bpftrace -e '
    tracepoint:sched:sched_process_fork
    /comm == \"postgres\"/
    { @forks = count(); }
    interval:s:1 { print(@forks); clear(@forks); }
    interval:s:12 { exit(); }
  '
"
```

While running the persistent connection benchmark, the output is:
```sh
Attaching 3 probes...
@forks: 0
@forks: 0
@forks: 11
@forks: 0
@forks: 0
@forks: 0
@forks: 0
@forks: 0
@forks: 0
@forks: 0
@forks: 0
@forks: 0
```

And during the connect-per-transaction run, the output is:
```sh
Attaching 3 probes...
@forks: 0
@forks: 496
@forks: 3561
@forks: 3276
@forks: 3300
@forks: 3398
@forks: 3425
@forks: 3435
@forks: 3392
@forks: 3582
@forks: 3605
@forks: 3048
```
We can see here all the thousands of backend processes being forked to handle transactions vs. just the one per connection/client. It is clear how looking at this can give us a strong indication of one source of overhead for our requests.

### Quantifying it: fork-to-ready latency

Counting forks tells us *that* connection churn is happening, but not *how expensive* each one actually is. We can get a real number by timestamping each fork (`tracepoint:sched:sched_process_fork`, keyed by the new child's pid) and pairing it with a `uretprobe` on `InitPostgres` - the function each freshly-forked backend calls to attach to shared memory and set up its catalog caches before it's ready to run a query. The elapsed time between the two is the genuine fork-to-ready latency, independent of anything `pgbench` self-reports:

```sh
PG_PID=$(docker inspect -f '{{.State.Pid}}' lab-postgres)
docker compose -f ../tools/analysis/compose.yml exec -T analysis bash -c "
  bpftrace -e '
    tracepoint:sched:sched_process_fork
    /comm == \"postgres\"/
    { @start[args.child_pid] = nsecs; }

    uretprobe:/proc/${PG_PID}/root/usr/lib/postgresql/16/bin/postgres:InitPostgres
    /@start[pid]/
    {
      @fork_to_ready_ns = hist(nsecs - @start[pid]);
      delete(@start[pid]);
    }
    interval:s:8 { exit(); }
  '
"
```
Run against the connect-per-transaction benchmark, this produced:
```
@fork_to_ready_ns: 
[256K, 512K)        6581 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@                        |
[512K, 1M)         11928 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[1M, 2M)            2771 |@@@@@@@@@@@@                                        |
[2M, 4M)              66 |                                                    |
[4M, 8M)               1 |                                                    |
```
Most forks go from "just forked" to "ready to accept a query" in roughly 0.25-1ms - squarely in line with `pgbench`'s own self-reported `average connection time` (~1.1-1.3ms) from the previous experiment. So two independent measurements - one from the client's own point of view, one from tracing the server process directly - land on the same number, which is good evidence that ~1ms is a reliable characteristic of the connection cost in our test setup here.

### Detecting it without touching Postgres at all: `accept()` rate

Both measurements above need Postgres's own symbols (`InitPostgres`) or at least its `comm` name. For a signal that would work purely from outside - watched continuously, without knowing in advance that connection churn is even the issue - the `accept` syscall on the postmaster's listening socket is enough on its own. (Worth checking `sys_enter_accept` vs `sys_enter_accept4` first - in our setup Postgres's calls showed up under plain `accept`, not the newer `accept4` variant; a quick unfiltered check of both settled it.)

```sh
PG_PID=$(docker inspect -f '{{.State.Pid}}' lab-postgres)
docker compose -f ../tools/analysis/compose.yml exec -T analysis bash -c "
  bpftrace -e '
    tracepoint:syscalls:sys_enter_accept
    /comm == \"postgres\"/
    { @accepts = count(); }
    interval:s:1 { print(@accepts); clear(@accepts); }
    '
"
```
Persistent connections:
```
@accepts: 11
@accepts: 0
@accepts: 0
@accepts: 0
@accepts: 0
@accepts: 0
```
Connect-per-transaction:
```
@accepts: 111
@accepts: 3461
@accepts: 3508
@accepts: 3148
@accepts: 3457
@accepts: 3474
@accepts: 3372
```
One `accept()` per client at the start, versus a sustained ~3000-3500/s for the whole run - the same story as the fork counts, but derived purely from a generic kernel tracepoint. This is the version of "detection" that matters most in practice: it needs no Postgres binary, no debug symbols, and no prior suspicion of what's wrong - a sustained, anomalous rate of `accept()` calls on a database's listening socket is, by itself, a strong enough signal to go looking for a connect-per-request antipattern, the same way an unexplained spike in `calls` on one query in `pg_stat_statements` was the tell for N+1.
