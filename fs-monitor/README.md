# fs-monitor

Simple program to monitor file system change events inside a given directory (and subdirectories). Using [fsnotify](github.com/fsnotify/fsnotify), which internally relies on `inotify` internally.

We start off with a directory watcher that logs the events. We could do something a little more interesting, like computing statistics for the different events, but the really interesting thing will probably be in seeing how `fsnotify` relies on `inotify` system calls and the Go netpoller (and what it uses internally, like `epoll`).
