FROM debian:12

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y \
        bpftrace \
        git \
        build-essential \
        clang \
        llvm \
        make \
        cmake \
        pkg-config \
        wget \
        curl \
        ca-certificates \
        file \
        procps \
        strace \
        linux-perf \
        && rm -rf /var/lib/apt/lists/*

WORKDIR /opt

CMD ["/bin/bash"]
