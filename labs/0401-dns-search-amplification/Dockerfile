FROM golang:1.23 AS build
WORKDIR /src
COPY fakedns.go ./
RUN CGO_ENABLED=0 go build -o /fakedns fakedns.go

FROM debian:bookworm-slim
COPY --from=build /fakedns /usr/local/bin/fakedns
CMD ["fakedns"]
