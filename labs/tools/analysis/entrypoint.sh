#!/usr/bin/env bash
set -e

# OrbStack's VM does not mount these by default. We mount them ourselves on
# every container start (each container gets its own mount namespace, so this
# can't be done once on the host and inherited) - requires --privileged.
mountpoint -q /sys/kernel/debug || mount -t debugfs debugfs /sys/kernel/debug 2>/dev/null || true
mountpoint -q /sys/kernel/tracing || mount -t tracefs tracefs /sys/kernel/tracing 2>/dev/null || true
mountpoint -q /sys/fs/bpf || mount -t bpf bpf /sys/fs/bpf 2>/dev/null || true

exec "$@"
