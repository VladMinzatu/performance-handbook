## Build docker image

```
docker build -t doc-pipeline:latest .
```

see local images:

```
docker images
```

## Run with docker

```
docker run --rm -p 8080:8080 doc-pipeline:latest
```
## Run with docker-compose (including monitoring)

Run:
```
docker compose up
```

Prometheus UI is at `localhost:9090`
And Grafana is at `localhost:3000`


To stop the application and tear down monitoring infra:
```
docker compose down
```
