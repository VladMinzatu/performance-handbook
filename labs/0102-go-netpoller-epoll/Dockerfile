FROM golang:1.23 AS build
WORKDIR /src
COPY server.go client.go ./
RUN go build -o /netpoll-server server.go
RUN go build -o /netpoll-client client.go

FROM debian:bookworm-slim AS server
COPY --from=build /netpoll-server /usr/local/bin/netpoll-server
CMD ["netpoll-server"]

FROM debian:bookworm-slim AS client
COPY --from=build /netpoll-client /usr/local/bin/netpoll-client
CMD ["netpoll-client"]
