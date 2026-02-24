# Continuous profiling with Pyroscope

We'll spin up Grafana Pyroscope and opentelemetry-collector-contrib with docker compose. And to collect the data, we'll need to run the [opentelemetry-ebpf-profiler](https://github.com/open-telemetry/opentelemetry-ebpf-profiler) (see toolset docs for setup instructions):

```
sudo ./ebpf-profiler -collection-agent=127.0.0.1:4317 -disable-tls
```

This setup produces the following view in Pyroscope:

![Pyroscope](assets/pyroscope.png)

We can see a big offender right away: around 90% of the on CPU time is taken by the DedupAndIndex function. It's time to take our profiling offline to investigate further.

[Next](03_pprof.md)
