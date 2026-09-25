FROM golang:1.23 AS build
WORKDIR /src
COPY storm.go threads.go ./
RUN CGO_ENABLED=0 go build -o /storm storm.go && \
    CGO_ENABLED=0 go build -o /threads threads.go

FROM debian:bookworm-slim
COPY --from=build /storm /usr/local/bin/storm
COPY --from=build /threads /usr/local/bin/threads
CMD ["storm"]
