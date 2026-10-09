FROM golang:1.23 AS build
WORKDIR /src
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /ingest main.go

FROM debian:bookworm-slim
COPY --from=build /ingest /usr/local/bin/ingest
CMD ["ingest"]
