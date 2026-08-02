Idea: organize labs around the systems performance debugging, tuning and understanding loop:
- Form a hypothesis based upon our current understanding
- Gather evidence meant to prove or disprove the hypothesis
- Analyze data and update understanding
- Repeat until system is sufficiently understood / well performing.

Tooling: experiments run in Docker (via OrbStack's Linux VM) using the
reusable infrastructure in [tools/](./tools/README.md) - a long-running
privileged "analysis" container (bpftrace, Inspektor Gadget, profilers,
benchmarking tools) plus a per-experiment compose file for the system under
test.

Backlog:
- CPU
- Scheduling
- Memory
- Storage
- Networking and protocols
  - connection pooling in different runtimes
- Concurrency & Synchronization
- Virtualization
- Containers & cgroup
- Databases
  - PG: Connection overhead / pooling (raw connections vs PgBouncer) — measures per-request connection setup cost and how pooling changes throughput under bursty load,
- Language runtimes and GC
- GPUs / accelerators
- Compilers
- NUMA
- Filesystems
- Distributed systems