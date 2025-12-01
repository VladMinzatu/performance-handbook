# Syscall Tracing & Runtime Security Tools

These tools inspect syscall activity and process behavior at runtime, enabling real-time observability, auditing, and threat detection. Unlike general-purpose profilers, they prioritize safety, policy enforcement, and clarity.

## sysdig

Trace system calls and events in real-time, similar to `strace`, but much more powerful.

Key Features:
- Captures detailed system activity: process, file, network, container events.
- Supports filters and scripting.
- Has both CLI and a library of "chisels" (scripts for analysis).
- Available in both open source and commercial versions.

Use Case: Audit syscalls and investigate what a process or container is doing.

Example usage:
```
sudo sysdig evt.type=open
sudo sysdig -p"%proc.name %evt.args" fd.name contains /etc
```

## Falco

Behavioral runtime security engine based on syscall tracing.

Key Features:
- Uses a rule engine to detect suspicious or abnormal activity.
- Built on top of Sysdig/eBPF.
- Comes with a rich default rule set (e.g., "shell spawned in container").
- Integrates well with Kubernetes and cloud-native environments.

Use Case: Monitor and alert on malicious or policy-violating behaviors in production.

Example Usage:
```
falco --rule-file falco_rules.yaml
```

Notes
- These tools often run as daemons or agents.
- They are especially relevant in production, Kubernetes, and cloud-native security.
- Falco is CNCF-graduated and widely adopted in DevSecOps workflows.
