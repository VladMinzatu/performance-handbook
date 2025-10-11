# ebpf-go-test

Trying out the local setup using the [Getting Started](https://ebpf-go.dev/guides/getting-started/) guide.

Build instructions:
```
go generate && go build
```

First we generate the `.o` files using bpf2go. These files are to be embedded during the build. The build won't work without this step if you've just checked out the repo.
Then we can build our binary using the `go build` command.

To run the program after updating user code:
```
go build && sudo ./ebpf-go-test
```

When iterating on the C code, bpf2go needs to be re-run by re-running the generate step, so to run in that case:
```
go generate && go build && sudo ./ebpf-go-stest
```
