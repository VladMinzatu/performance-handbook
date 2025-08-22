# sig-counter

Minimalist program to demonstrate signal handling in Go: a simple time ticker/counter with display and reset functionality.

## Go perf tools

Let's first look at a CPU profile, just to see what it looks like. If we run the process and run a couple of `kill -USR1 $PID` and `kill -USR2 $PID` commands, at the end we collect the following CPU profile:
```
(pprof) top10
Showing nodes accounting for 10ms, 100% of 10ms total
      flat  flat%   sum%        cum   cum%
      10ms   100%   100%       10ms   100%  runtime.kevent
         0     0%   100%       10ms   100%  runtime.findRunnable
         0     0%   100%       10ms   100%  runtime.mcall
         0     0%   100%       10ms   100%  runtime.netpoll
         0     0%   100%       10ms   100%  runtime.park_m
         0     0%   100%       10ms   100%  runtime.schedule
```

Not really surprisingly, there isn't much going on. As far as the profile can tell, we're spending all our CPU time blocked, waiting for timers and signals.

The Go runtime implements both timers and signal handling by sitting in a platform specific system call, which is `kevent` in macOS (it would probably be an `epoll*` on Linux).

Next, let's have a look at a trace under similar "load":
![Trace](assets/trace-sig.webp)

Much like matter in our universe, actual activity here is few and far between. 