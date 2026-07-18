FROM ubuntu:24.04

# This image is the "outside observer": eBPF tracing, profiling and load
# generation tools. It is not meant to host the system under test itself -
# language runtimes / app code for a given experiment belong in that
# experiment's own service/image, not here.

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    # eBPF / kernel tracing
    bpftrace \
    # general Linux introspection
    strace \
    sysstat \
    procps \
    htop \
    lsof \
    numactl \
    # benchmarking / load generation
    stress-ng \
    fio \
    sysbench \
    wrk \
    # networking
    iproute2 \
    iputils-ping \
    dnsutils \
    tcpdump \
    netcat-openbsd \
    # scripting
    python3 \
    jq \
    curl \
    git \
    less \
    vim \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Inspektor Gadget standalone CLI (native arch, CO-RE via BTF - no kernel headers needed)
RUN set -eux; \
    case "$(dpkg --print-architecture)" in \
        amd64) ig_arch=amd64 ;; \
        arm64) ig_arch=arm64 ;; \
        *) echo "unsupported architecture for ig" >&2; exit 1 ;; \
    esac; \
    ig_version="$(curl -fsSL https://api.github.com/repos/inspektor-gadget/inspektor-gadget/releases/latest | grep '"tag_name"' | head -1 | cut -d '"' -f4)"; \
    curl -fsSL "https://github.com/inspektor-gadget/inspektor-gadget/releases/download/${ig_version}/ig-linux-${ig_arch}-${ig_version}.tar.gz" \
      | tar -xz -C /usr/local/bin ig; \
    chmod +x /usr/local/bin/ig

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["bash"]
