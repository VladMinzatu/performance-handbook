
## Setup
```
docker network create otel-net
```

## Running Pyroscope

We will use docker:
```
docker run -it --name pyroscope --network otel-net -p 4040:4040 grafana/pyroscope:latest
```

## Running OLTP collector (contrib) to pass through the OLTP to Pyroscope

We will use docker here as well:

```
docker run --rm --name otelcol --network otel-net -v "$(pwd)/collector-config.yaml":/etc/otelcol/config.yaml -p 4317:4317 otel/opentelemetry-collector-contrib:latest --config  /etc/otelcol/config.yaml  --feature-gates=service.profilesSupport
```

## Running the ebpf-profiler

We will use the docker build to create the `./ebpf-profiler` binary:
```
git clone git@github.com:open-telemetry/opentelemetry-ebpf-profiler.git
cd opentelemetry-ebpf-profiler/

make agent
```
And this creates the `ebpf-profiler` binary, which we can now run:
```
sudo ./ebpf-profiler -collection-agent=127.0.0.1:4317 -disable-tls
```
