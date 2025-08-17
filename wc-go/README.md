# wc-go

This is a simplified version of the standard Unix command-line utility [wc](https://man7.org/linux/man-pages/man1/wc.1.html). It doesn't support all the flags and arguments. In fact, it will always just print out the number of lines, number of words, and number of characters in the input. The input can be a file, or stdin in case no file path is provided as an argument.

Instead, though, it does support some other arguments (the `-p` flag), which indicate strategies for how we will process the input. (for our tests). The strategies we are testing are:
- `scanner` - creates a bufio.Scanner directly on the os.File (or stdin) to process the input line by line and allocate and return a string per line to be processed in successive calls to scanner.Text. This would be the most common, idiomatic approach that you would see in the wild - and probably appropriate in most cases.
- `upfront` - calls io.ReadAll or os.ReadFile depending on the case - loading the whole contents in memory before processing.
- `buffering` - manually fill the buffer that eventually contains all the data, loaded in chunks, and then do the processing.
- `mmap` - (only works with files) use the mmap system call to get read only access to the file data as a byte slice and then do the processing on it.
- `mmap2` - same as mmap, but we will process the file data using a scanner, just to see the effect.

In all strategies except for the `scanner` and `mmap2` ones, once the data is available as a byte slice, we do all processing directly on the bytes and avoid any extra heap allocations.

## Time tests

Let's start with the very basics: just about the simplest test we can run - timing the process run. I will use a file called `shakespeare100.txt`, which is formed by concatenating a file containing the complete works of Shakespeare 100 times over (this results in a file about 500MB in size - I did not check it in, obviously). A shakespeare.txt file can easily be found online and concatenating it can be done like so:

```
for i in {1..100};do cat shakespeare.txt >> shakespeare100.txt; done
```
So, running the times, first for the standard `wc` program:
```
% time wc shakespeare100.txt 
12418500 89958800 543647500 shakespeare100.txt
wc shakespeare100.txt  1.00s user 0.05s system 99% cpu 1.061 total
```

And now for the different versions of our own program:
```
% time ./wc-go -p scanner ~/shakespeare100.txt 
12418500	89958800	531229000
./wc-go -p scanner ~/shakespeare100.txt  4.35s user 0.15s system 96% cpu 4.655 total
```

```
% time ./wc-go -p upfront ~/shakespeare100.txt
12418500	89958800	531229000
./wc-go -p upfront ~/shakespeare100.txt  3.61s user 0.13s system 99% cpu 3.742 total
```

```
% time ./wc-go -p buffering ~/shakespeare100.txt
12418500	89958800	531229000
./wc-go -p buffering ~/shakespeare100.txt  3.94s user 0.33s system 104% cpu 4.094 total
```

```
 % time ./wc-go -p mmap ~/shakespeare100.txt
12418500	89958800	531229000
./wc-go -p mmap ~/shakespeare100.txt  3.64s user 0.04s system 99% cpu 3.676 total
```

```
% time ./wc-go -p mmap2 ~/shakespeare100.txt
12418500	89958800	531229000
./wc-go -p mmap2 ~/shakespeare100.txt  4.35s user 0.08s system 102% cpu 4.338 total
```

It's fair to say we didn't break new ground here, but what can we notice in these initial results?
- First, using `mmap` may feel clever, but it's probably not providing much benefit for our use case (it performs just about the same as reading everything up front). In fact, afaik, the original `wc` implementation uses `read()` syscalls to stream through the file in big contiguous chunks. Makes sense why that would be the most efficient way to go, when also considering that mmap could incur page faults. We'll need to dig into that a bit more.
- Looking at the time differences between the runs of different `wc-go` variants, it is quite clear to see that there is a significant difference between the variants that perform additional allocations for each line before processing it and those who don't. Isn't it more than you'd expect? Maybe. In any case, interesting to observe if you've programmed in languages that steer you towards instantiating objects very liberally.
- And why are **all** `wc-go` variants considerably slower than the original? I would put that down to the Go runtime overhead and perhaps lack of some other compiler optimizations. We'll try to uncover that in more detail as well.

## Go perf tools

We will use github.com/pkg/profile to profile our application runs in various runs. Enabling profiling is a simple matter of adding the following line (or similar, depending on the type of profile) to the top of our `main` function and then running the process again and then running the appropriate `go tool` on the output:
```
defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
```
Example of runing the pprof tool:
```
go tool pprof -http=:8080 cpu.pprof 
```
Although the graph view is the most interesting to look at in interactive mode, it's not nicely shareable here, so I will paste the output of `top10` where appropriate.

### CPU Profiles
- scanner:
```
Showing nodes accounting for 4.55s, 98.48% of 4.62s total
Dropped 24 nodes (cum <= 0.02s)
Showing top 10 nodes out of 51
      flat  flat%   sum%        cum   cum%
     4.22s 91.34% 91.34%      4.22s 91.34%  syscall.syscall
     0.11s  2.38% 93.72%      0.11s  2.38%  runtime.pthread_cond_wait
     0.11s  2.38% 96.10%      0.11s  2.38%  runtime.pthread_kill
     0.07s  1.52% 97.62%      0.07s  1.52%  runtime.usleep
     0.04s  0.87% 98.48%      0.04s  0.87%  runtime.kevent
         0     0% 98.48%      4.22s 91.34%  bufio.(*Scanner).Scan
         0     0% 98.48%      4.23s 91.56%  github.com/VladMinzatu/performance-handbook/wc-go/cmd.Run
         0     0% 98.48%      4.23s 91.56%  github.com/VladMinzatu/performance-handbook/wc-go/cmd.Run.func1
         0     0% 98.48%      4.23s 91.56%  github.com/VladMinzatu/performance-handbook/wc-go/processing.(*InputProcessor).Run
         0     0% 98.48%      4.23s 91.56%  github.com/VladMinzatu/performance-handbook/wc-go/processing.process
```
- upfront:
```
Showing nodes accounting for 3.62s, 99.18% of 3.65s total
Dropped 11 nodes (cum <= 0.02s)
Showing top 10 nodes out of 30
      flat  flat%   sum%        cum   cum%
     1.41s 38.63% 38.63%      1.41s 38.63%  unicode/utf8.DecodeRune
     0.70s 19.18% 57.81%      0.70s 19.18%  github.com/VladMinzatu/performance-handbook/wc-go/processing.isSpace (inline)
     0.56s 15.34% 73.15%      2.01s 55.07%  github.com/VladMinzatu/performance-handbook/wc-go/processing.(*WordCountProcessor).Process
     0.55s 15.07% 88.22%      1.21s 33.15%  github.com/VladMinzatu/performance-handbook/wc-go/processing.(*CharacterCountProcessor).Process
     0.34s  9.32% 97.53%      3.57s 97.81%  github.com/VladMinzatu/performance-handbook/wc-go/processing.processBytes
     0.04s  1.10% 98.63%      0.04s  1.10%  runtime.memclrNoHeapPointers
     0.02s  0.55% 99.18%      0.02s  0.55%  runtime.pthread_cond_signal
         0     0% 99.18%      3.62s 99.18%  github.com/VladMinzatu/performance-handbook/wc-go/cmd.Run
         0     0% 99.18%      3.62s 99.18%  github.com/VladMinzatu/performance-handbook/wc-go/cmd.Run.func1
         0     0% 99.18%      3.62s 99.18%  github.com/VladMinzatu/performance-handbook/wc-go/processing.(*InputProcessor).Run
```
- buffering:
```
Showing nodes accounting for 4030ms, 99.26% of 4060ms total
Dropped 14 nodes (cum <= 20.30ms)
Showing top 10 nodes out of 37
      flat  flat%   sum%        cum   cum%
    1500ms 36.95% 36.95%     1500ms 36.95%  unicode/utf8.DecodeRune
     850ms 20.94% 57.88%      850ms 20.94%  github.com/VladMinzatu/performance-handbook/wc-go/processing.isSpace (inline)
     450ms 11.08% 68.97%     2100ms 51.72%  github.com/VladMinzatu/performance-handbook/wc-go/processing.(*WordCountProcessor).Process
     380ms  9.36% 78.33%     1080ms 26.60%  github.com/VladMinzatu/performance-handbook/wc-go/processing.(*CharacterCountProcessor).Process
     360ms  8.87% 87.19%     3540ms 87.19%  github.com/VladMinzatu/performance-handbook/wc-go/processing.processBytes
     200ms  4.93% 92.12%      200ms  4.93%  runtime.usleep
     120ms  2.96% 95.07%      120ms  2.96%  runtime.memclrNoHeapPointers
     100ms  2.46% 97.54%      100ms  2.46%  syscall.syscall
      70ms  1.72% 99.26%       70ms  1.72%  runtime.memmove
         0     0% 99.26%      100ms  2.46%  bufio.(*Reader).Read
```
- mmap:
```
Showing nodes accounting for 3.69s, 99.46% of 3.71s total
Dropped 5 nodes (cum <= 0.02s)
      flat  flat%   sum%        cum   cum%
     3.69s 99.46% 99.46%      3.70s 99.73%  github.com/VladMinzatu/performance-handbook/wc-go/processing.processBytes
         0     0% 99.46%      3.71s   100%  github.com/VladMinzatu/performance-handbook/wc-go/cmd.Run
         0     0% 99.46%      3.71s   100%  github.com/VladMinzatu/performance-handbook/wc-go/cmd.Run.func1
         0     0% 99.46%      3.71s   100%  github.com/VladMinzatu/performance-handbook/wc-go/processing.(*InputProcessor).Run
         0     0% 99.46%      3.71s   100%  github.com/VladMinzatu/performance-handbook/wc-go/processing.runWithMmapOnFile
         0     0% 99.46%      3.71s   100%  github.com/spf13/cobra.(*Command).Execute (inline)
         0     0% 99.46%      3.71s   100%  github.com/spf13/cobra.(*Command).ExecuteC
         0     0% 99.46%      3.71s   100%  github.com/spf13/cobra.(*Command).execute
         0     0% 99.46%      3.71s   100%  main.main
         0     0% 99.46%      3.71s   100%  runtime.main
```
- mmap2:
```
Showing nodes accounting for 4500ms, 98.68% of 4560ms total
Dropped 23 nodes (cum <= 22.80ms)
Showing top 10 nodes out of 55
      flat  flat%   sum%        cum   cum%
    4000ms 87.72% 87.72%     4000ms 87.72%  runtime.memmove
     100ms  2.19% 89.91%      100ms  2.19%  runtime.pthread_cond_signal
     100ms  2.19% 92.11%      100ms  2.19%  runtime.usleep
      90ms  1.97% 94.08%       90ms  1.97%  runtime.pthread_cond_wait
      80ms  1.75% 95.83%       80ms  1.75%  runtime.kevent
      80ms  1.75% 97.59%       80ms  1.75%  runtime.pthread_kill
      50ms  1.10% 98.68%       50ms  1.10%  runtime.madvise
         0     0% 98.68%     4000ms 87.72%  bufio.(*Scanner).Scan
         0     0% 98.68%     4000ms 87.72%  bytes.(*Reader).Read
         0     0% 98.68%     4040ms 88.60%  github.com/VladMinzatu/performance-handbook/wc-go/cmd.Run
```

More than anything, these results show us how the CPU profiler works. A CPU profile samples where the program spends CPU cycles. I.e., importantly, that is not wall time that is being reported. 

That explains why the `scanner` version spends 92% of its time doing `Read` syscalls (likely successively reading in chunks of 64KB at a time or so), while the percentages in the other profiles are dominated by the processing of the text data.

In general, a long running system call will likely not even show up in the profile (if there is one or a small number of them), because during that time, the thread is parked and not using CPU cycles.
