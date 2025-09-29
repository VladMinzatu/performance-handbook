# bpflist

Trying out the [bpflist](https://github.com/iovisor/bcc/blob/master/tools/bpflist.py) tool, which shows info on running BPF programs.

For example, running:
```
sudo ./bpflist
```

while we have `./bashreadline` running as well, prints out:
```
PID    COMM             TYPE  COUNT
1      systemd          prog  19
20330  python           map   1
20330  python           prog  1
```

We see `./bashreadline` reported as having 1 program installed and 1 map open. And also systemd which has installed 19 BPF programs.

