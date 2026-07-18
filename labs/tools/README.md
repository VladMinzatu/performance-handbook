# Lab tooling

Reusable Docker infrastructure for running experiments entirely inside Linux
(via OrbStack's VM), so the eBPF-based tools work the same way they would on
a real Linux box, without needing a separate manually-managed VM.

## Design

- **`analysis/`** - one long-running, privileged container with the
  observability/benchmarking toolkit (bpftrace, BCC tools, `perf`,
  Inspektor Gadget's `ig`, `psql`/`pgbench`, `stress-ng`, `fio`, `sysbench`,
  Python for crunching results, etc). Started once and left running; you
  `exec` into it per experiment. It runs with `pid: host` so it can see and
  trace processes belonging to *any* other container on the Docker host, not
  just ones on the same network.
- **`labnet`** - a Docker network created by `analysis/compose.yml`. Every
  experiment's "system under test" (Postgres, an app under load, etc) joins
  this network as `external`, so the analysis container can reach it by
  service name.
- **Per-experiment `compose.yml`** - each experiment directory (e.g.
  `labs/001-postgres-index-impact/`) defines only the system under test. See
  `examples/postgres/compose.yml` for the template.

The analysis image is deliberately just the "outside observer" - tracing,
profiling, load generation. Language runtimes or app code being studied
belong in that experiment's own service/image, not in the analysis image.

## One-time setup

```sh
docker compose -f labs/tools/analysis/compose.yml up -d --build
```

This builds the image, creates the `labnet` network, and starts
`lab-analysis` in the background (`sleep infinity` - it has nothing to do
until you exec into it). Re-run with `--build` whenever you change the
Dockerfile.

## Per-experiment workflow

1. Start the system under test, joined to `labnet`:
   ```sh
   docker compose -f labs/tools/examples/postgres/compose.yml up -d
   ```
2. Get a shell in the analysis container:
   ```sh
   docker compose -f labs/tools/analysis/compose.yml exec analysis bash
   ```
   The repo root is mounted at `/workspace`, so scripts and captured results
   land back on your Mac.
3. From that shell: `psql -h postgres -U postgres -d labdb`,
   `bpftrace -e '...'`, `ig run trace_open:latest --containername lab-postgres`,
   etc. Find the target PID(s) with `pgrep postgres` (visible because of
   `pid: host`). Verified example - trace the files a query actually opens:
   ```sh
   ig run trace_open:latest --containername lab-postgres
   ```
   and, in another shell in the same container, run a query. Output includes
   real backend file access, e.g. `base/16384/2601` for a system catalog.
4. Tear down the system under test when done; leave `analysis` running for
   the next experiment:
   ```sh
   docker compose -f labs/tools/examples/postgres/compose.yml down
   ```

## Notes / tradeoffs

- **`--privileged` + `pid: host`**: the simplest reliable way to get eBPF
  tracing working (fine-grained capabilities are possible but fiddly and
  version-dependent). Fine for a local, single-user sandbox; not something
  to carry into a shared or production setting.
- **`perf`**: installed via `linux-tools-generic`, which targets a stock
  Ubuntu kernel version, not OrbStack's custom kernel. Basic
  `perf stat`/`perf record` generally still work since the `perf_event_open`
  syscall ABI is stable across versions, but treat symbolization/PMU-event
  availability as best-effort and verify per experiment.
- **No kernel headers needed**: OrbStack's kernel ships BTF
  (`/sys/kernel/btf/vmlinux`), so bpftrace/BCC/`ig` all use CO-RE and don't
  need `/usr/src` or `/lib/modules` mounted in.
- **debugfs/tracefs/bpffs**: OrbStack doesn't mount these by default, and
  mounts don't propagate between containers, so `analysis`'s entrypoint
  mounts them itself on every start (needs `--privileged`).
- Architecture: OrbStack on Apple Silicon runs an arm64 Linux VM. The image
  installs `bpftrace`/`ig` from arch-appropriate native builds - avoid
  amd64-only images (they'll run under emulation, which works but is slower
  and not worth it for tools you'll use constantly).
- **`ig` container enrichment**: needs the whole host filesystem mounted at
  `/host` (`HOST_ROOT=/host`), not just the Docker socket - it reads
  containerd's per-container runtime state (`config.json` etc.) directly off
  disk to map PIDs to container names/images. The socket alone isn't enough.
