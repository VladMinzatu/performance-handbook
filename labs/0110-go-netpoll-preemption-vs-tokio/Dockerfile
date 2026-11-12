FROM golang:1.27.3 AS build
WORKDIR /src
COPY goserver.go client.go ./
RUN CGO_ENABLED=0 go build -o /goserver goserver.go && CGO_ENABLED=0 go build -o /client client.go

FROM debian:bookworm-slim
COPY --from=build /goserver /client /usr/local/bin/
