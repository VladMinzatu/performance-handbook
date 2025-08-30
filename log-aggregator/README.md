# log-aggregator

In production systems, log capture is usually handled transparently by the runtime environment (e.g., Docker, containerd, or systemd), which redirects each process’s stdout/stderr into a managed logging pipeline. In this project, instead of relying on that layer, we experiment with different low-level techniques for local log aggregation — such as pipes, Unix domain sockets, and files — to better understand the trade-offs of various IPC mechanisms before forwarding logs into a distributed system.
