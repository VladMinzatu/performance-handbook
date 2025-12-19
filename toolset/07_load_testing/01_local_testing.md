# Local Load Testing Tools

## ab

`ab` is a simple command-line HTTP benchmarking tool distributed with the Apache HTTP Server, used for basic load and latency testing.

Key Features:

- Generates concurrent HTTP requests
- Reports throughput and latency statistics
- Minimal configuration and setup
- Widely available on Unix-like systems

Use Cases:

- Quick, ad hoc endpoint testing
- Baseline performance checks
- Legacy environments where other tools are unavailable

Example usage:

```
ab -n 10000 -c 100 http://localhost:8080/
```

Notes

- Limited to simple request patterns
- Generally superseded by hey and wrk

## hey

`hey` is a lightweight HTTP load generation tool written in Go, designed as a modern, more usable alternative to ApacheBench.

Key Features:

- Simple CLI interface
- Supports request bodies and headers
- Generates concurrent HTTP load
- Single static binary

Use Cases:

- Developer-level load testing
- Smoke tests and regressions
- Quick throughput and latency measurements

Example usage:

```
hey -n 10000 -c 100 http://localhost:8080/
```

Notes

- Not suitable for modeling complex user behavior
- Focused on simplicity and speed

## wrk/wrk2

`wrk` is a high-performance HTTP benchmarking tool capable of generating significant load using a single machine; `wrk2` extends it with constant-rate request generation.

Key Features:

- Event-driven, multithreaded architecture
- Lua scripting for request customization
- High throughput with low overhead
- wrk2 supports fixed request rates

Use Cases:

- Stress testing HTTP servers
- Measuring maximum throughput
- Microservice performance benchmarking

Example usage:

```
wrk -t4 -c100 -d30s http://localhost:8080/
wrk2 -R 10000 -t4 -c100 -d30s http://localhost:8080/
```

Notes:

- Emphasizes raw load rather than realism
- Requires careful interpretation of results
