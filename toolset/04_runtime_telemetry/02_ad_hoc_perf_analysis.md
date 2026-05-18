# Ad hoc performance analysis tools

Tools that use eBPF for runtime performance analysis, e.g. ad hoc inspection:
- [Inspektor Gadget](https://inspektor-gadget.io/)
- [Pixie](https://px.dev/)
- [Parca](https://www.parca.dev/)
- [Cilium Tetragon](https://tetragon.io/)
- [BCC Tools](https://github.com/iovisor/bcc)
- [bpftrace](https://github.com/bpftrace/bpftrace)


Typical eBPF observability stack:
- [Inspektor Gadget](https://inspektor-gadget.io/) for low-overhead production observability and operational debugging, with standardized diagnostics and Kubernetes-native flows.
- [bpftrace](https://github.com/bpftrace/bpftrace) for ad-hoc performance analysis
- Custom CO-RE applications for maximum flexibility and production use cases
