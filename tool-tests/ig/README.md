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

with the possibility to filter by container or pid:
```
--containername <name>
--containerid <id>
--pid <pid>
```