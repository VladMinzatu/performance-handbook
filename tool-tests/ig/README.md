# Inspektor Gadget

## Using Docker

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


with the possibility to filter by container or pid:
```
--containername <name>
--containerid <id>
--pid <pid>
```