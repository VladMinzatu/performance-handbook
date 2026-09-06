FROM golang:1.23 AS build
WORKDIR /src
COPY workerpool.go ./
RUN go build -o /workerpool workerpool.go

FROM debian:bookworm-slim
COPY --from=build /workerpool /usr/local/bin/workerpool
CMD ["workerpool"]
