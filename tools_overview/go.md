# Go Tools for profiling and observability

## Benchmarks

Benchmarks execute a certain code segment a number of times in order to get a stable estimate for the execution time of that code segment.

The easiest and standard way to write benchmarks in Go is to use the built-in functionality in the `testing` package, i.e. writing a specific type of test (inside a `*_test.go` file) and then executing it with a `go test -bench=.` command.

Benchmark functions have a signature of the form `func Benchmark*(*testing.B)`. `b.N` will typically be used inside the function and determines the number of iterations. The output from running `go test -bench=.` will tell us how many times the code segment under test was run and what was the average time it took.

Other flags that are supported are `-benchmem` to include memory allocations and `-benchtime=5` to set the duration.

## Profilers

`pprof` is Go's built-in profiling tool that can be used via `net/http/pprof` or `runtime/pprof`.

#### Option1: Using the `net/http/pprof` runtime server

Just start your app with `net/http/pprof` imported and start a server if the app doesn't already start one

```
import _ "net/http/pprof"
import "net/http"
...
func main() {
  go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
  }
}
```

This will make endpoints `localhost:6060/debug/pprof/*` accessible. The following profiles are now possible:

- `debug/pprof/heap` - Memory allocations snapshot
- `debug/pprof/profile?seconds=30` - CPU profile (defaults to 30s)
- `debug/pprof/goroutine` - Goroutine dump
- `debug/pprof/block` - blocking events
- `debug/pprof/mutex` - Mutex contention
- `debug/pprof/threadcreate` - OS thread creation events

You can call them directly, or you can still use the cli tool:

```
go tool pprof http://localhost:6060/debug/pprof/profile
```

#### Option2: Using `runtime/pprof`

They way to work with profilers typically involves these general steps:

- modify your code to start a profiler (this will typically involve pointing it to a file that collects the profile data)
- run the code
- use the `go tool pprof -http:8080 <filename>` to inspect the output

This can be combined nicely with benchmarks when appropriate (e.g. `go test -bench='.' -cpuprofile='cpu.prof' -memprofile='mem.prof'`), but sometimes you'll probably want to test large code segments or an entire application in a production environment or something that resembles that.

Note: `go test ...` would also work with those \*profile options.

#### When to use which one?

Interacting with the Go profiling infrastructure can be achieved with either the `runtime/pprof` approach or the `net/http/pprof` approach and they are functionally nearly equivalent. You may want to combine the two: have `net/http/pprof` for production on-demand diagnostics (profiles are triggered via HTTP) and use the `runtime/pprof` approach in tests, benchmarks and offline/local diagnostics (it requires manual control and setup, but gives more control and can be used to test critical sections of code in isolation, avoid noise in profiles, profile tests and scritps, etc.).

Using the `runtime/pprof` approach is not meant for production, as in some cases the overhead is significant.

With either of these approaches, you can use multiple profiler types supported by Go:

### CPU profiler

Will capture statistics about the on-CPU time of the code by interrupting the program every 10ms through interrupts and taking a stack trace.

The profiling is started by placing this in the function that includes the code you want to profile:

```
...
pprof.StartCPUProfile(f)
defer pprof.StopCPUProfile()
```

or with https://github.com/pkg/profile :

```
defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
```

### Memory profiler

Collects data about the memory allocations per stack trace. The go runtime itself does this by recording the stack trace that lead to allocations (at a certain sample rate, which is tunable, capturing all is possible by setting `runtime.MemProfileRate`), instead of using an OS interrupt to capture CPU cycles.

Example using https://github.com/pkg/profile

```
defer profile.Start(profile.MemProfile, profile.MemProfileRate(1), profile.ProfilePath(".")).Stop()
```

Alternatively, `pprof.WriteHeapProfile(f)` could be used to do a heap dump at a point in time. `go tool pprof mem.prof` can be used on the output.

### Goroutine profiler

Shows current goroutines and their stack traces and can reveal goroutine leaks, deadlocks and concurrency issues.

### Block profiler

Will capture off-CPU time spent waiting on channels and mutexes (but not sleep, I/O, GC). The statistics will show cumulatie delays per stack trace.

### Mutex profiler

The same as the block profiler, but only looks at mutexes, but excluding channels.

Subtle difference, though. The mutex profiler compiles statistics per stack trace _causing the blocking_, as opposed to being blocked (which is what the block profiler does). Generally, you'd want to use both profilers together.

### Goroutine profiler

Collects data about number of goroutines per stack trace.

WARNING: this is a stop the world profiler, no sampling mechanism.

But it can be useful for detecting goroutine leaks or diagnose why a program might hang.

## Tracing: Go's built-in runtime Scheduler Tracer

Tracing is the recording of timestamped events. (this is useful at the go application level, the same way distributed tracing is useful for understanding performance at a distributed system level)

Go's scheduler tracer lives in the `runtime/trace` package and is a low-level, highly detailed tracer. (it's separate from pprof)

The built in runtime tracer captures scheduler, GC, contention, syscal etc. events as well as user-defined trace regions and tasks (via `trace.Log`, `trace.WithRegion`, `trace.NewTask`) (see src/runtime/trace.go)

You want to run this to answer questions such as:

- Why is this goroutine not running immediately?
- What is blocking the scheduler?
- Why is GC taking so long?
- Why is the CPU underutilized?
- What bottlenecks are there?

To run it:

```
...
trace.Start(f)
defer trace.Stop()
//...application logic
...
```

When done, run the trace viewer with:

```
go tool trace trace.out
```

This can expose issues not captured by the profilers mentioned above.

To run, e.g. using https://github.com/pkg/profile :

```
defer profile.Start(profile.TraceProfile, profile.Path(".")).Stop()
```

Can also be exported to Prometheus as metrics accessible through the metrics endpoint (e.g. https://github.com/MadhavJivrajani/gse - essentially it runs a go program with `GODEBUG=schedtrace=10 <binary>` and then it scans the stderr for "SCHED" and then parses those traces to extract metrics and pushes them to prometheus).

Tracing is not meant to be used in production (except maybe in short bursts) as the overhead it adds is high.

## Observability

For higher level observability, check https://github.com/open-telemetry/opentelemetry-go

This one is designed for production use cases, obviously.

# Experiment Ideas

- Optimizing memory allocations (heap vs stack) - benchmarks

- stack allocations escaping to the heap

- Memory leak scenarios

- Goroutine leaks

- performance channel sync vs locking primitives (channel sync is seamless as part of the regular scheduling, whereas locking is extra and expensive operatio - demo it)

- goroutine vs mutex use cases

## References

https://blog.logrocket.com/benchmarking-golang-improve-function-performance/

https://www.youtube.com/watch?v=7hg4T2Qqowk

https://www.youtube.com/watch?v=nok0aYiGiYA

https://stackademic.com/blog/profiling-go-applications-in-the-right-way-with-examples

https://blog.devgenius.io/profiling-in-go-finding-and-fixing-performance-bottlenecks-868e5c7e929b
