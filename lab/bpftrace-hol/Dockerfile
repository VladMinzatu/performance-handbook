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

# Fetch the Hands-on Lab
# RUN wget https://home.fburl.com/~jonhaslam/bpftrace-hol.tar.gz .
# RUN tar -xzf bpftrace-hol.tar.gz && rm bpftrace-hol.tar.gz

# WORKDIR /opt/bpftrace-hol

# # Build the load generators
# RUN make || true

CMD ["/bin/bash"]
