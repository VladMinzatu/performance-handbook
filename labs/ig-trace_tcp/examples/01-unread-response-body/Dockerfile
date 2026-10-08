FROM golang:1.23 AS build
WORKDIR /src
COPY server.go client.go ./
RUN CGO_ENABLED=0 go build -o /server server.go && CGO_ENABLED=0 go build -o /client client.go

FROM debian:bookworm-slim
COPY --from=build /server /client /usr/local/bin/
