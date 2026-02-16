# ---------- build stage ----------
FROM golang:1.25-alpine AS builder

WORKDIR /doc-pipeline

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -o doc-pipeline ./


# ---------- runtime stage ----------
FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /doc-pipeline/doc-pipeline /app/doc-pipeline
COPY --from=builder /doc-pipeline/data/shakespeare.txt /app/data/shakespeare.txt

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/doc-pipeline"]