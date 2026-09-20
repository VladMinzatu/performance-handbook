FROM golang:1.23 AS build
WORKDIR /src
COPY hog.go ./
RUN CGO_ENABLED=0 go build -o /hog hog.go

FROM debian:bookworm-slim
COPY --from=build /hog /usr/local/bin/hog
COPY run-both.sh /usr/local/bin/run-both.sh
RUN chmod +x /usr/local/bin/run-both.sh
CMD ["hog"]
