FROM golang:1.27 AS build
ENV GOEXPERIMENT=simd
WORKDIR /src
COPY go.mod kernels.go kernels_simd.go kernels_test.go ./
RUN go test -c -o /simd.test

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      python3 python3-numpy linux-perf binutils \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /simd.test /usr/local/bin/simd.test
COPY numpy/bench.py /bench.py
CMD ["sleep", "infinity"]
