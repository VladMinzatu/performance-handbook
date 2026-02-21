# Runtime telemetry

In this repo we spin up containers for Prometheus and Grafana along with our application to display our custom metrics along with some Go runtime internals and some system metrics from cAdvisor:

![Grafana Screenshot](assets/grafana.png)

To begin with, we're generating 100 docs per second and running them through the whole pipeline. Each pipeline stage has 10 workers and a buffer of size 100. Everything's running smoothly and comfortably.

