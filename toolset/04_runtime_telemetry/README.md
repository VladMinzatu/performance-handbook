# Runtime telemetry

This directory contains the configuration to launch a local OTel stack of otel-collector - prometheus - grafana - jaeger.

To launch:
```
docker compose up -d
```

To check logs:
```
docker compose logs -f otel-collector
```

The UIs are available at:
- Jaeger UI: http://localhost:16686
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000

To tear down:
```
docker compose down
```
