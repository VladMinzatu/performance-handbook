FROM golang:1.23 AS build
WORKDIR /src
COPY counter.go .
RUN go build -o /counter counter.go

FROM debian:bookworm-slim
COPY --from=build /counter /usr/local/bin/counter
CMD ["counter"]
