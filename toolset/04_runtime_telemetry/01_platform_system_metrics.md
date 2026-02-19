# Platform system metrics

The platform should ideally provide plug-and-play auto-instrumentation that can be tapped into to add to the operational dashboard with minimal effort, when new services are created or new infra is provisioned.

Thus, they are useful for the teams, but also for the platform org to have an overview and own alerting.

The main components are:

- Applications (auto-)instrumentation using Otel libs and/or agents
- Metrics collection and discovery: something like Prometheus, scraping k8s service endpoints, node metrics via node-exporter and cgroup metrics via kubelet+cAdvisor
- Logs: centralised
- Trace backend like jaeger
- Dashboarding and alerting (e.g. Grafana, Alertmanager)
- Cost reporting with kubecost and AWS cost explorer
- Network and deep kernel observability (e.g. Cilium) - eBPF gives low-overhead kernel-level hooks and higher fidelity than purely procfs-based scraping.
- Continous profiling (e.g. pyroscope)

Under the hood, metrics come from:

- cgroups: container resource accounting (CPU time, memory usage, blkio) is exposed to userspace via cgroup files containing counters - [cAdvisor](https://github.com/google/cadvisor) / kubelet read these to compute per-container stats. The cgroup metrics are sourced from the kernel via /sys/fs/cgroup/. Docker creates cgroups, but resource accounting is entirely done by the Linux kernel, not Docker. cAdvisor will get container names, labels, associated PIDs to cgroups from Docker, but the metrics from the kernel files.
- [Node-exporter](https://github.com/prometheus/node_exporter) is a Prometheus exporter for operating system and hardware metrics. Written in go and running as a lightweight daemon, it exposes low-level system and hardware metrics of Linux hosts, i.e. node level (bare metal or VMs) to monitoring systems - most commonly Prometheus. It is commonly deployed with k8s (as a DaemonSet), but that's only one use case. The data comes from procfs (/proc) and sysfs (/sys) to get CPU, memory. This is the classic, default approach for host and process metrics.
- For exposing NVIDIA GPU metrics, [prometheus-dcgm](https://github.com/NVIDIA/dcgm-exporter) can be used. It exposes GPU metrics as an exporter for Prometheus leveraging [NVIDIA DCGM](https://developer.nvidia.com/dcgm).
- and then there's the k8s api for kubernetes specific stuff, eBPF for low-overhead, high-resolution kernel telemetry and the application level instrumentation.

References:

- https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/hostmetricsreceiver/README.md
- https://opentelemetry.io/docs/languages/go/instrumentation/
- https://opentelemetry.io/docs/zero-code/go/autosdk/
