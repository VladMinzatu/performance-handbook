# The PIDs controller: refusing new processes instead of killing existing ones

Uses the shared lab infrastructure in [tools/](../tools/README.md) - the
`analysis` container's Inspektor Gadget (`ig`) for `trace_exec`, and
`bpftrace` for the syscall-level view that `trace_exec` can't give.

## Background

A container's `--pids-limit` is a cgroup v2 `pids.max` value: a cap on the
number of tasks that may exist in the cgroup at once. It is a different
kind of enforcement from a memory limit. Crossing `memory.max` makes the
kernel pick a victim and kill it. Crossing `pids.max` kills nothing: the
`fork()`/`clone()` that would create the extra task simply fails with
`EAGAIN` ("Resource temporarily unavailable"), and everything already
running carries on. It is a refusal at the point of creation, not a
punishment after the fact.

Two details shape everything this lab measures:

- **The limit counts tasks, not processes.** Every thread is a task, so a
  multithreaded runtime can exhaust `pids.max` without ever calling
  `exec`. A limit of 20 is 20 threads-plus-processes combined.
- **A denied `fork` never reaches `exec`.** The usual pattern is
  `fork()` then, in the child, `exec()`. When `pids.max` refuses the first
  step there is no child, so there is no exec to observe. A tool that
  traces `execve` sees only the attempts that *succeeded*; the refusals
  show up as events that never happened, which is a hard thing to notice
  in a trace. The kernel does keep its own tally - `pids.events` in the
  cgroup has a `max` counter of how many times a fork was refused - and
  the failed `clone` return value is visible at the syscall boundary.

Docker's own surface for this is even thinner than for OOM: there is no
`OOMKilled`-style field in `docker inspect` for "a fork was denied". The
container stays `running` with exit code 0 the whole time. The only
evidence is in the cgroup files and in whatever the workload itself logs
when its fork fails.

## Hypotheses

**Prediction 1 - `trace_exec` records exactly the successes, and the
count saturates at the limit.** A workload that keeps spawning
long-lived children under a low `pids.max` should produce a burst of
exec events, then stop: the number of successful execs is bounded by
`pids.max` minus what was already running, and no further events appear
however long the workload keeps trying. The denied attempts are
invisible to `trace_exec`; they show as a flat line after the burst.

**Prediction 2 - the refusals are recoverable evidence elsewhere.**
Three independent sources should agree on the number of denied forks:
`pids.events` (`max` counter), a `bpftrace` probe on the `clone`
syscall's return value (`-EAGAIN`, value `-11`), and the workload's own
error log. Where they disagree is itself a finding - for example a
runtime that retries internally will make one logical failure show up as
several denied `clone` calls.

**Prediction 3 - refusal is not a kill: nothing dies, and the limit is
about count, not rate.** The container's main process keeps running,
already-spawned children are untouched, and `docker inspect` shows
nothing unusual (`running`, exit code 0, no OOM flag). As soon as some
children exit and `pids.current` drops below `pids.max`, new forks
succeed again. This is the contrast with a memory limit: a memory
limit ends something, a pids limit only says "not now".

**Prediction 4 - threads count, so a thread-heavy workload trips the
limit with zero execs.** A Go program blocking many goroutines in
syscalls needs many OS threads. Under a low `pids.max` it should hit the
limit through thread creation alone: `pids.events` `max` increments and
`trace_exec` shows nothing at all. This makes "trace the execs" the wrong
tool for a whole class of pids-limit incidents, and shows how the failure
looks from inside a Go process (the runtime cannot create an OS thread).

## Setup

Build and start the lab's containers:
```sh
docker compose -f compose.yml up -d --build
```
- A fork-storm container with a low `pids.max`, running a workload that
  keeps spawning long-lived child processes and logs each success and
  each failure - the case for Predictions 1-3.
- A thread-heavy container with a low `pids.max`, running a Go workload
  that parks many goroutines in blocking syscalls - the case for
  Prediction 4.

Confirm the limit and watch it fill, from inside the container's own
cgroup view:
```sh
docker exec lab-pids-storm cat /sys/fs/cgroup/pids.max /sys/fs/cgroup/pids.current
docker exec lab-pids-storm cat /sys/fs/cgroup/pids.events
```

Watch the successful execs, attributed by container:
```sh
docker exec lab-analysis ig run trace_exec:latest --containername lab-pids-storm
```

Count the refused forks at the syscall boundary (the `clone`/`clone3`
return value is `-11` for `EAGAIN`):
```sh
PID=$(docker inspect --format '{{.State.Pid}}' lab-pids-storm)
docker exec lab-analysis sh -c "bpftrace -e 'tracepoint:syscalls:sys_exit_clone,tracepoint:syscalls:sys_exit_clone3 /args.ret == -11/ { @denied[comm] = count(); }'"
```
(Filter to the container's cgroup rather than all `clone` calls host-wide;
the experiments show the exact predicate.)

Cross-check what Docker itself reports - the comparison Prediction 3 is
actually about:
```sh
docker inspect lab-pids-storm --format '{{.State.Status}} exitcode={{.State.ExitCode}} pids-limit={{.HostConfig.PidsLimit}}'
```

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down
```
