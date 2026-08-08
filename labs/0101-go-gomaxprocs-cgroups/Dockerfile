FROM golang:1.23 AS build
WORKDIR /src
COPY main.go .
RUN go build -o /cpuburn main.go

FROM debian:bookworm-slim
COPY --from=build /cpuburn /usr/local/bin/cpuburn
CMD ["cpuburn"]
