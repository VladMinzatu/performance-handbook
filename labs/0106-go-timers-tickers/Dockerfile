FROM golang:1.23 AS build
WORKDIR /src
COPY timer.go .
RUN go build -o /timer timer.go

FROM debian:bookworm-slim
COPY --from=build /timer /usr/local/bin/timer
CMD ["timer"]
