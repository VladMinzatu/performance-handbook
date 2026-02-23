# Continuous profiling with Pyroscope

We'll spin up Grafana Pyroscope and opentelemetry-collector-contrib with docker compose. And to collect the data, we'll need to run the [opentelemetry-ebpf-profiler](https://github.com/open-telemetry/opentelemetry-ebpf-profiler) (see toolset docs for setup instructions):

```
sudo ./ebpf-profiler -collection-agent=127.0.0.1:4317 -disable-tls
```

This setup produces the following view in Pyroscope:

![Pyroscope](assets/pyroscope.png)

This matches what we saw before: around 90% of the on CPU time is taken by the cosine calculation. Except the function shown here is the caller of `cosine`: `DedupAndIndex`.

This is probably just down to inlining done on compilation. We can check this by blocking the inlining of `cosine` with a `//go:noinline`. After rebuilding the image and restarting the whole stack, we see:

![Pyroscope noinling](assets/pyroscope_noinline.png)

Indeed, everything seems to be working as expected and we get our flamegraphs from the ebpf-profiler.
