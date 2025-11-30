//go:build ignore
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <linux/ptrace.h>

char LICENSE[] SEC("license") = "GPL";

// Tunables: max stack depth (number of 64-bit entries)
#define MAX_STACK_FRAMES 64

struct event {
    __u32 pid;
    __u32 cpu;
    __u64 ts_ns;
    __s32 user_stack_bytes;   // return from bpf_get_stack for user stack (or negative)
    __s32 kernel_stack_bytes; // return from bpf_get_stack for kernel stack (or negative)
    __u64 user_stack[MAX_STACK_FRAMES];
    __u64 kernel_stack[MAX_STACK_FRAMES];
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24); // 16MB ring buffer (tunable)
} events SEC(".maps");

SEC("perf_event")
int sample(struct bpf_perf_event_data *ctx)
{
    struct event *e;
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = (__u32)(pid_tgid >> 32);
    __u32 cpu = bpf_get_smp_processor_id();

    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e)
        return 0;

    e->pid = pid;
    e->cpu = cpu;
    e->ts_ns = bpf_ktime_get_ns();

    int uret = bpf_get_stack(ctx, &e->user_stack, sizeof(e->user_stack), BPF_F_USER_STACK);
    e->user_stack_bytes = uret;

    int kret = bpf_get_stack(ctx, &e->kernel_stack, sizeof(e->kernel_stack), 0);
    e->kernel_stack_bytes = kret;

    bpf_ringbuf_submit(e, 0);
    return 0;
}
