# Production Load Testing Tools

## k6

`k6` is a modern, scriptable load testing tool focused on developer workflows, CI/CD integration, and realistic traffic modeling.

Key Features:

- JavaScript-based test scripts
- Scenario-driven execution model
- Built-in assertions and thresholds
- Strong integration with observability systems
- Supports local, distributed, and cloud execution

Use Cases:

- Pre-production load and stress testing
- Performance regression testing in CI
- SLA and SLO validation
- Large-scale distributed load testing

Example usage:

```
k6 run test.js
```

Notes:

- Considered a de facto industry standard for API load testing
- Managed cloud offering simplifies large-scale execution

## Locust

Locust is a distributed load testing framework that models user behavior using Python code, emphasizing realism and flexibility.

Key Features:

- Python-based user behavior definitions
- Distributed master/worker architecture
- Web UI for test control and reporting
- Stateful and workflow-driven testing

Use Cases:

- Complex user journey simulation
- Load testing of systems with state
- Large-scale tests requiring custom logic
- Python-centric engineering environments

Example usage:

```
locust -f locustfile.py
```

Notes:

- More operational overhead than single-binary tools
- Particularly effective when testing complex workflows
