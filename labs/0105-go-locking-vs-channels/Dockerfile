FROM golang:1.23 AS build
WORKDIR /src
COPY counter.go pingpong.go ./
RUN go build -o /counter counter.go
RUN go build -o /pingpong pingpong.go

FROM debian:bookworm-slim AS counter
COPY --from=build /counter /usr/local/bin/counter
CMD ["counter"]

FROM debian:bookworm-slim AS pingpong
COPY --from=build /pingpong /usr/local/bin/pingpong
CMD ["pingpong"]
