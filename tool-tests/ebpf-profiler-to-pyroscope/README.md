
## Running Pyroscope

We will use docker:
```
docker run -it -p 4040:4040 grafana/pyroscope
```

## Running the collector
We will use docker here as well:

```
docker run --rm -p 4317:4317 -v "$(pwd)/collector-config.yaml":/etc/otelcol/config.yaml:ro otel/opentelemetry-collector-contrib:v0.137.0 --config /etc/otelcol/config.yaml --feature-gates=service.profilesSupport

```

## Running the ebpf-profiler

We will use the docker build to create the `./ebpf-profiler` binary:
```
git clone git@github.com:open-telemetry/opentelemetry-ebpf-profiler.git
cd opentelemetry-ebpf-profiler/

make agent
```
And this creates the `ebpf-profiler` binary.
