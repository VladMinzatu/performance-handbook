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
