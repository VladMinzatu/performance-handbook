FROM golang:1.27.3 AS build
WORKDIR /src
COPY backend.go client.go ./
RUN CGO_ENABLED=0 go build -o /backend backend.go && CGO_ENABLED=0 go build -o /client client.go

FROM debian:bookworm-slim
COPY --from=build /backend /client /usr/local/bin/
