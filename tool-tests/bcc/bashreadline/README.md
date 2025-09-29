# bashreadline

Trying out the [bashreadline](https://github.com/iovisor/bcc/blob/master/tools/bashreadline.py) tool, which prints bash commands from all running shells on the system.

Can be run like so (in e.g. `/usr/share/bcc/tools`):
```
sudo ./bashreadline
```

Example output:
```
TIME      PID     COMMAND
16:52:52  18942   cd
16:52:54  18942   whoami
16:53:11  18942   wc -l .bashrc 
```

