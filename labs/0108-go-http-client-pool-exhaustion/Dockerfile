FROM golang:1.23 AS build
WORKDIR /src
COPY inventory.go edge.go loadgen.go ./
RUN go build -o /inventory inventory.go
RUN go build -o /edge edge.go
RUN go build -o /loadgen loadgen.go

FROM debian:bookworm-slim
COPY --from=build /inventory /edge /loadgen /usr/local/bin/
