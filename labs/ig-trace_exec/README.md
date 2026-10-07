# `ig trace_exec`

Inspektor Gadget's `trace_exec` reports every `execve()` in a container:
every new program started, by whom, and with which arguments. Each example under
[examples/](./examples) is a small program that misuses process creation,
run once with a bad setting and once with a good one, with `trace_exec`
used to detect the bad case. Uses the shared lab infrastructure in
[tools/](../tools/README.md) (`lab-analysis` runs `ig`).

## What it sees

One event per `execve()`, captured in the kernel when the call happens, so
nothing is missed however short-lived the process is. Fields worth
knowing:

| Field | What it tells you |
|---|---|
| `runtime.containerName` | Which container the exec happened in |
| `proc.comm`, `proc.pid` | The new program's name and its PID |
| `proc.parent.comm`, `proc.parent.pid` | Who started it - follow this to build process trees |
| `args` | Full command line (truncated to the gadget's buffer size) |
| `error` | Why a failed exec failed (`ENOENT`, `EACCES`, ...) - failed execs are hidden unless `--ignore-failed=false` |
| `upper_layer` | `true` if the binary lives in the container's writable layer, not the image: something downloaded or copied in at runtime |
| `exepath`, `cwd` | Binary path and working directory (need `--paths`) |

What it doesn't see: a `fork()` with no `exec()` (prefork worker pools,
Python `multiprocessing` with the fork start method), new threads, and
process exits. It also can't tell you how long a process ran. It records
starts.

## Why it matters in production

Short-lived processes are easy to miss. `ps`, `top` and `docker top` take
snapshots, and a process that lives for a millisecond shows up in some of
them and not others, so a snapshot gives no rate and no history. Its CPU
is charged to the container, but not to any process you can find. Typical
sources:
- application code shelling out per request (`subprocess`, `exec.Command`,
  `system()`), often through `sh -c`
- exec-based health/readiness probes and sidecars running scripts on a
  timer
- entrypoint and wrapper scripts, cron jobs, crash-looping children
- unexpected programs: shells, package managers, or binaries dropped into
  the container at runtime (`upper_layer=true`)

## Usage

```sh
# stream every exec in one container
docker exec lab-analysis ig run trace_exec:latest -c <container>
# include failed execs (missing binaries, permission errors)
docker exec lab-analysis ig run trace_exec:latest -c <container> --ignore-failed=false
# readable parent / child / command line (args are escaped in column output)
docker exec lab-analysis sh -c "ig run trace_exec:latest -c <container> -o json \
  | jq -r '[.proc.parent.comm, .proc.comm, .args] | @tsv'"
# exec rate: count over 10s, grouped by parent and program
docker exec lab-analysis sh -c "ig run trace_exec:latest -c <container> -t 10 -o json \
  | jq -r '[.proc.parent.comm, .proc.comm] | @tsv' | sort | uniq -c"
# only binaries that aren't part of the image
docker exec lab-analysis ig run trace_exec:latest -c <container> -F upper_layer==true
```
Without `-c`, it traces every container on the host. Add `--host` to
include host processes.

## Examples

1. [Shelling out per request](./examples/01-shell-out-per-request/README.md) -
   a service that pipes each request through `sh -c sha256sum`, against
   the same work done in-process.
