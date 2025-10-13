#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 10240);
    __type(key, __u32);
    __type(value, __u64);
} ip_counters SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u32);
    __type(value, __u8);
} blocklist SEC(".maps");

SEC("xdp")
int xdp_block_count_prog(struct xdp_md *ctx) {
    void *data_end = (void *)(long)ctx->data_end;
    void *data = (void *)(long)ctx->data;

    // parse ethernet
    struct ethhdr *eth = data;
    if ((void*)(eth + 1) > data_end)
        return XDP_PASS;

    if (eth->h_proto != __constant_htons(ETH_P_IP))
        return XDP_PASS;

    struct iphdr *ip = (void*)eth + sizeof(*eth);
    if ((void*)(ip + 1) > data_end)
        return XDP_PASS;

    __u32 src = ip->saddr;

    // if source in blocklist => drop
    __u8 *blocked = bpf_map_lookup_elem(&blocklist, &src);
    if (blocked && *blocked) {
        return XDP_DROP;
    }

    // increment per-IP packet counter (64-bit)
    __u64 *cnt = bpf_map_lookup_elem(&ip_counters, &src);
    __u64 one = 1;
    if (cnt) {
        __sync_fetch_and_add(cnt, 1);
    } else {
        bpf_map_update_elem(&ip_counters, &src, &one, BPF_ANY);
    }

    return XDP_PASS;
}

char _license[] SEC("license") = "GPL";