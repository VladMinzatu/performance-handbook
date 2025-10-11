# ebpf-go-test

Trying out the local setup using the [Getting Started](https://ebpf-go.dev/guides/getting-started/) guide.

Build instructions:
```
go generate
go build
```

First we generate the `.o` files using bpf2go. These files are to be embedded during the build. The build won't work without this step.
Then we can build our binary and run it like so:
