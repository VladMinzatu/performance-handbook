# Lab tooling

Reusable Docker infrastructure for running experiments entirely inside Linux
(via OrbStack's VM), so the eBPF-based tools work the same way they would on
a real Linux box, without needing a separate manually-managed VM.

## Design

- **`analysis/`** - one long-running, privileged container that is purely
  the "agent" doing the observing/load-generating: `bpftrace`, Inspektor
  Gadget's `ig`, `stress-ng`, `fio`, `sysbench`, `wrk`, `tcpdump`, and general
  Linux introspection (`strace`, `sysstat`, `htop`, `lsof`, `numactl`).
  Started once and left running; you `exec` into it per experiment. It runs
  with `pid: host` so it can see and trace processes belonging to *any*
  other container on the Docker host, not just ones on the same network.
  Deliberately does **not** include client tools for a specific system under
  test (e.g. `psql`) - those already ship in that system's own image (see
  below), and pulling them into `analysis` would blur "the thing observing"
  with "the thing being observed."
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
3. From the `analysis` shell, observe: `bpftrace -e '...'`,
   `ig run trace_open:latest --containername lab-postgres`, etc. Find the
   target PID(s) with `pgrep postgres` (visible because of `pid: host`).
   To generate activity, use the system-under-test's own client, e.g.
   `docker exec -it lab-postgres psql -U postgres -d labdb` or
   `docker exec lab-postgres pgbench ...` - the official `postgres` image
   already ships `psql`/`pgbench`/`pg_isready`. Verified example - trace the
   files a query actually opens:
   ```sh
   # in the analysis container:
   ig run trace_open:latest --containername lab-postgres
   # in another shell, on the host:
   docker exec lab-postgres psql -U postgres -d labdb -c 'select 1;'
   ```
   Output includes real backend file access, e.g. `base/16384/2601` for a
   system catalog.
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
- **No `perf`**: Ubuntu's `linux-tools-generic` package installs a
  version-pinned `perf` that only works if a matching
  `linux-tools-<exact-kernel-version>` package exists - it doesn't for
  OrbStack's custom-built kernel, so `perf` fails outright rather than just
  being imprecise (verified: `WARNING: perf not found for kernel ...`). Not
  installed here since it'd be dead weight; real hardware-counter profiling
  needs a real Linux host/VM with a matching distro kernel.
- **No BCC tools (`bpfcc-tools`)**: overlaps with `bpftrace` (both are eBPF
  tracing frontends) and adds ~276MB for prebuilt commands like
  `opensnoop-bpfcc`/`biolatency-bpfcc` that you can express as a `bpftrace`
  one-liner when needed - which also fits better with writing/understanding
  the probe yourself rather than running a canned script.
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
