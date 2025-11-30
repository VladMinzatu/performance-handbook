# libbpf for CO-RE applications

On a fresh Ubuntu installation, install system dependencies:

```
sudo apt update && sudo apt upgrade -y

sudo apt install -y \
  build-essential clang llvm git cmake pkg-config make \
  libelf-dev libmnl-d01ev zlib1g-dev libzstd-dev \
  libbpf-dev linux-tools-$(uname -r)bpfto linux-headers-$(uname -r)
```

Install libbpf from source:

```
git clone https://github.com/libbpf/libbpf.git
cd libbpf/src

make -j"$(nproc)"

sudo make install PREFIX=/usr/local
```

## ebpf-go for CO-RE applications in Go

For Linux headers (for ebpf-go and bpf2go) this will probably be needed:

```
sudo ln -s /usr/include/aarch64-linux-gnu/asm /usr/include/asm
```

The tutorial for setting up a new ebpf-go application that uses the `bpf2go` tool can be found here: https://ebpf-go.dev/guides/getting-started/

If the C source of the ebpf program requires an `#include "vmlinux.h"` file for BTF, the `go generate` step will need to create it.
For this, the `gen.go` file would need to include a line for it, like in the example e.g.

```
package ebpf

//go:generate bash -c "bpftool btf dump file /sys/kernel/btf/vmlinux format c > bpf/vmlinux.h"
//go:generate go tool bpf2go -tags linux profile bpf/profile.c
```
