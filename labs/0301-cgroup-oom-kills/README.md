# OOM kills under cgroup memory limits: what actually gets killed, and why

Uses the shared lab infrastructure in [tools/](../tools/README.md) -
unlike most other labs, the `analysis` container's Inspektor Gadget
(`ig`) isn't an optional add-on here: it's the only
thing in this lab that can say *which* process died and *why*, not just
that something did.

## Background

A container's `--memory` limit is a cgroup v2 `memory.max` value. Cross
it, and the kernel doesn't fail the allocation that pushed things over -
it invokes the OOM killer, scoped to that cgroup, to pick a victim and
free space by force. Victim selection is a "badness" score, dominated by
how much resident memory (RSS) a process is actually holding - not which
process is oldest, which is PID 1, or which one "caused" the immediate
allocation.

Docker's own visibility into this is thin. `docker inspect` exposes a
single `OOMKilled` boolean and the exit code (`137`, `SIGKILL`) - enough
to know *that* the cgroup hit its limit, nothing about which process was
picked, how much memory it held, or whether the container's main process
was even the one killed. A container running more than one process can
have a background task silently killed while its entrypoint keeps
running - `OOMKilled: true` and `Status: running` at the same time, a
combination that looks contradictory until you know that's exactly what
it means. `ig`'s `trace_oomkill` gadget instruments the kernel's own OOM
path directly, and reports the victim's `comm`/`pid`/resident page count,
the container it belongs to, and the process whose allocation triggered
the check - the full picture Docker's own tooling can't reconstruct
after the fact.

One general memory-management fact matters for building any workload to
demonstrate this: `make([]byte, N)` (or `malloc` - this isn't
Go-specific) doesn't cost N bytes of real memory the moment it's called.
Freshly allocated anonymous memory is backed by a single shared
system-wide zero page until something actually writes to it; reading
zeros needs no private copy. A "memory hog" that allocates and never
writes to what it allocated won't pressure a cgroup's `memory.max` at
all, no matter how much virtual memory it appears to hold - it has to
touch every page it wants counted.

## Hypotheses

**Prediction 1 - `ig trace_oomkill` gives a precise, attributable answer
where Docker's own signals are ambiguous.** A container running two
processes under one shared memory limit can end up `OOMKilled: true`
while its `Status` stays `running`, because the process actually killed
wasn't the container's PID 1. `docker logs`/`ps` alone can't even
distinguish *which* of two identically-named processes was the one that
died. `ig trace_oomkill` resolves both: the container it happened in, the
exact victim PID, and its resident page count at the moment of the kill.

**Prediction 2 - victim selection tracks actual memory held, not process
identity or role.** In a container running two instances of the same
binary under one memory limit, one held to a small, fixed footprint and
the other left to grow unbounded, the OOM killer should reliably pick the
unbounded one - regardless of which was started first, which one's
allocation happened to trigger the check, or that both share the same
process name.

**Prediction 3 - "the container had an OOM event" and "the container
died from OOM" are different claims, and which one is true depends on
which process gets hit.** A single-process container's OOM kill always
ends the container - there's nothing else running. A multi-process
container's OOM kill might not: if the killed process is a background
task rather than PID 1, the container can carry on, `OOMKilled: true`
recorded permanently against a container that never actually stopped.
Whether this generalizes to every case is worth checking rather than
assuming - `memory.oom.group`, if set, changes the picture entirely by
killing every process in the cgroup together instead of just the single
worst offender.

## Setup

Build and start the lab's containers:
```sh
docker compose -f compose.yml up -d --build
```
- A single-process container, memory-limited, running a workload that
  actively grows its resident memory until it's killed - the direct case
  for Prediction 1.
- A multi-process container under one shared memory limit: one process
  held to a small, fixed footprint, one left to grow unbounded - the case
  for Predictions 2 and 3.

Watch for the kill directly, attributed by container and process:
```sh
docker exec lab-analysis ig run trace_oomkill:latest
```

Cross-check what Docker itself reports for the same event - this is the
comparison Prediction 1 is actually about:
```sh
docker inspect <container> --format '{{.State.Status}} exitcode={{.State.ExitCode}} OOMKilled={{.State.OOMKilled}}'
```

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down
```
