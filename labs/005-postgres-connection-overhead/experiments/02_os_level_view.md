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
We can see here all the thousands of backend processes being forked to handle transactions vs. just the one per connection/client.
