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


## Running Inspektor Gadget Using Docker

For inspecting local processes or containers, we can run `ig` with docker by running OCI gadgets, e.g. to trace exec calls on the host:
```
docker run --rm -it \
  --privileged \
  --pid=host \
  -v /:/host \
  ghcr.io/inspektor-gadget/ig:latest \
  run trace_exec
```

producing output e.g.:
```
RUNTIME.CONTAINERNAME                                                                                                      COMM                    PID        TID TID        TTY         ARGS                                                                            ERROR
nginx-test                                                                                                                 ls                    28755      28755 28246      0           /usr/bin/ls\u00a0/    
```

**Note**: On a Mac, this setup will typically capture events for running containers, or in the Linux VM, but obviously, not what is happening on the Mac host. So to reproduce the output above, we have an nginx container running (`docker run --rm -d --name nginx-test nginx`) and when the ig inspection is running, we can trigger an exec with e.g. `docker exec nginx-test ls /`.

You can also by container or pid:
```
--containername <name>
--containerid <id>
--pid <pid>
```