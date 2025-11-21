
## Running Pyroscope

We will use docker:
```
docker run -it -p 4040:4040 grafana/pyroscope
```

## Running the ebpf-profiler

We will use the docker build to create the `./ebpf-profiler` binary:
```
git clone git@github.com:open-telemetry/opentelemetry-ebpf-profiler.git
cd opentelemetry-ebpf-profiler/

make agent
```
And this creates the `ebpf-profiler` binary.
