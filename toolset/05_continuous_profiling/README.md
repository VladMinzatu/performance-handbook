## Running everything end2end

Pyroscope and Parca are popular tools for collecting and visualising continuous profiling data. We will focus on Pyroscope, mainly for Go applications here.

For Go applications, Pyroscope uses [pyroscope-go](https://github.com/grafana/pyroscope-go) for integration (push mode), which in turn uses the standard `runtime/pprof` package to collect profiling data at runtime.
For more details see the [integration docs](https://grafana.com/docs/pyroscope/latest/configure-client/language-sdks/go_push/) and the official Go profiling [docs](https://go.dev/doc/diagnostics#profiling).

It thus supports most profiling types that the go tool pprof supports: https://grafana.com/docs/pyroscope/latest/introduction/profiling-types/. Go tracing is out of scope, tracing data is very rich and reserved for offline and focused analysis.

> **Overhead**: That said, some overhead considerations should not be ignored (recall runtime/pprof was covered under offline profiling). Some profile types are heavier than others, sampling rates and sampling windows are configurable and fleet amortization should also be used (also, push is not the only option, agent setups using Grafana agent/alloy are possible).

Additionally, the [opentelemetry-ebpf-profiler](https://github.com/open-telemetry/opentelemetry-ebpf-profiler) can be used in conjunction with Pyroscope as well. This leverages ebpf to support mixed stacktraces between runtimes - stacktraces go from Kernel space through unmodified system libraries all the way into high-level languages.

## Data encoding

Pyroscope supports the most popular industry standard encoding formats for ingestion:

- [pprof profile](github.com/google/pprof/profile)
- [OpenTelemetry profiling signal](https://github.com/open-telemetry/opentelemetry-proto/pull/534) - still experimental
- Speedscope

### 1. Set up pyroscope and collector

Set up pyroscope and the collector:

```
docker compose up -d
```

This launches:

- Pyroscope UI on http://localhost:4040
- OTel Collector listening for OTLP/gRPC profiling data on localhost:4317

To view logs, run:

```
docker compose logs -f
```

To stop everything:

```
docker compose down
```

### 2. Run the ebpf-profiler

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

## Alternative: run interactively

### Setup

```
docker network create otel-net
```

### Running Pyroscope

We will use docker:

```
docker run -it --name pyroscope --network otel-net -p 4040:4040 grafana/pyroscope:latest
```

### Running OLTP collector (contrib) to pass through the OLTP to Pyroscope

We will use docker here as well:

```
docker run --rm --name otelcol --network otel-net -v "$(pwd)/collector-config.yaml":/etc/otelcol/config.yaml -p 4317:4317 otel/opentelemetry-collector-contrib:latest --config  /etc/otelcol/config.yaml  --feature-gates=service.profilesSupport
```

### Running the ebpf-profiler

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
