FROM golang:1.23 AS build
WORKDIR /src
COPY worker.go delay-server.go ./
RUN go build -o /worker worker.go
RUN go build -o /delay-server delay-server.go

FROM debian:bookworm-slim AS worker
COPY --from=build /worker /usr/local/bin/worker
CMD ["worker"]

FROM debian:bookworm-slim AS delay-server
COPY --from=build /delay-server /usr/local/bin/delay-server
CMD ["delay-server"]
