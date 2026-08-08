# Go's GOMAXPROCS vs. container CPU limits

Uses the shared lab infrastructure in [tools/](../tools/README.md), but
only for the optional OS-level step - the two Go containers here don't
need `labnet` or any network reachability at all. Unlike the Postgres
labs, nothing connects to them over a network; the OS-level view observes
them by PID.

## Background

Go's scheduler decides how many OS threads it's willing to run application
goroutines on simultaneously via `GOMAXPROCS`, which defaults to
`runtime.NumCPU()` - the number of CPUs in the process's current
scheduling affinity mask, as reported by `sched_getaffinity`.

A container CPU limit set the common way - Docker/Compose's `cpus:` (or
`--cpus`) - does not touch that affinity mask at all. It configures the
cgroup's CFS *bandwidth* controller instead: a quota of CPU-time per
scheduling period (e.g. "200ms of CPU time per 100ms period" for a 2-CPU
limit), enforced by throttling, not by hiding CPUs from the process. Only
`--cpuset-cpus` (a different cgroup controller) changes what
`sched_getaffinity` reports. The result: a Go binary in a container capped
at 2 CPUs, on a host with 8, still sees and schedules for 8 - and finds
out about the real limit only by repeatedly getting throttled.

## Hypotheses

**Prediction 1 - Go's default GOMAXPROCS ignores cgroup CPU quotas
entirely.** A Go binary started in a container with `cpus: "2"` set,
without an explicit `GOMAXPROCS`, should report `GOMAXPROCS`/`NumCPU()`
equal to the *host's* full core count, not 2 - because quota-based limits
never touch the affinity mask Go actually reads.

**Prediction 2 - that mismatch causes real throttling and a real
throughput loss, not just a cosmetic wrong number.** Running a CPU-bound
workload with exactly `GOMAXPROCS` busy goroutines - i.e. the wrong,
oversized default - inside a 2-CPU-quota container should get throttled
heavily by the kernel's CFS bandwidth controller (visible directly in the
cgroup's `cpu.stat`: high `nr_throttled` relative to `nr_periods`, and
large cumulative `throttled_usec`), and should complete meaningfully less
actual work (ops/sec of the CPU-bound workload) than the identical binary
run with `GOMAXPROCS` set to match the real limit.

**Prediction 3 (stretch) - the throttling is visible as excessive
scheduling activity at the OS level, independent of `cpu.stat`.** With
more runnable threads than the quota can actually sustain, the kernel has
to preempt and switch between them far more than it would with a
correctly-sized thread count. Tracing `sched_switch` events for each
container's threads should show a substantially higher context-switch
rate for the oversized-GOMAXPROCS case than for the correctly-sized one,
for the same amount of useful work.

## Setup

Build and start both containers - identical image, identical 2-CPU quota,
differing only in whether the `GOMAXPROCS` environment variable is set:
```sh
docker compose -f compose.yml up -d --build
```
`cpuburn-default` leaves `GOMAXPROCS` unset (the broken default);
`cpuburn-fixed` sets `GOMAXPROCS=2` to match the container's real
entitlement. Each prints its own detected `GOMAXPROCS`/`NumCPU` once at
startup, then throughput (`ops/sec`) once a second:
```sh
docker logs -f lab-go-cpuburn-default
docker logs -f lab-go-cpuburn-fixed
```

To inspect the cgroup CPU-throttling counters directly:
```sh
docker exec lab-go-cpuburn-default cat /sys/fs/cgroup/cpu.stat
docker exec lab-go-cpuburn-fixed cat /sys/fs/cgroup/cpu.stat
```
`nr_periods` is how many scheduling periods have elapsed; `nr_throttled`
is how many of those periods this cgroup got throttled in; `throttled_usec`
is cumulative time spent throttled, waiting for the next period's quota.

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down -v
```
