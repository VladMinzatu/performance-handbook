# wc-go

This is a simplified version of the standard Unix command-line utility [wc](https://man7.org/linux/man-pages/man1/wc.1.html). It doesn't support all the flags and arguments. In fact, it will always just print out the number of lines, number of words, and number of characters in the input. The input can be a file, or stdin in case no file path is provided as an argument.

Instead, though, it does support some other arguments (the `-p` flag), which indicate strategies for how we will process the input. (for our tests). The strategies we are testing are:
- `scanner` - creates a bufio.Scanner directly on the os.File (or stdin) to process the input line by line and allocate and return a string per line to be processed in successive calls to scanner.Text. This would be the most common, idiomatic approach that you would see in the wild - and probably appropriate in most cases.
- `upfront` - calls io.ReadAll or os.ReadFile depending on the case - loading the whole contents in memory before processing.
- `buffering` - manually fill the buffer that eventually contains all the data, loaded in chunks, and then do the processing.
- `mmap` - (only works with files) use the mmap system call to get read only access to the file data as a byte slice and then do the processing on it.
- `mmap2` - same as mmap, but we will process the file data using a scanner, just to see the effect.

In all strategies except for the `scanner` and `mmap2` ones, once the data is available as a byte slice, we do all processing directly on the bytes and avoid any extra heap allocations.

To start off, let's look at the timing results for these different implementations -> [Next](./experiments/01_timing.md)

## Linux and BPF Tools

Let's continue looking into the `mmap` version behaviour. As we saw, whatever file reads happen (on page faults), they are invisible to the Go runtime and thus, also to the tracer. As far as it can tell, our goroutine is going along, never blocked, reading from memory, processing and writing back to memory. But we suspect that there's much more to the story - we know there's more complex things going on under the hood. How can we get some visibility into that?

### Standard Linux perf tools

On Linux, it makes sense to start with `perf` tools. How can it help us out here? We can run:
```
sudo perf stat -e page-faults,minor-faults,major-faults ./wc-go -p mmap ~/shakespeare100.txt 
```

and it will give us the following output:
```
 Performance counter stats for './wc-go -p mmap shakespeare100.txt':

             8,605      page-faults                                                           
             8,601      minor-faults                                                          
                 1      major-faults                                                          

       3.831913027 seconds time elapsed

       3.719923000 seconds user
       0.123196000 seconds sys

```
We're getting some interesting insights that we didn't have before, like the user vs. sys time. But more to our point, we get a clear counter for the number of minor faults vs major faults. This may look a bit surprising at first. Major faults happen when disk I/O is done, but a minor fault only means page tables being updated in memory, because the data itself for the page is already cached, so this is much faster.

What's more, if we run the same command again, we get the following output:
```
Performance counter stats for './wc-go -p mmap shakespeare100.txt':

             8,614      page-faults                                                           
             8,614      minor-faults                                                          
                 0      major-faults                                                          

       3.617099471 seconds time elapsed

       3.578519000 seconds user
       0.049048000 seconds sys
```
You can probably guess what happened here: the data is still cached from the previous run because the kernel hasn't had a reason to clear those pages, so we're getting a nice boost in our runtime (remember that `mmap` is meant for repeated random access in large files primarily - we just happened to be doing it in separate runs of our process...and we're also not accessing randomly).

But getting back to the first run, we only paid for one major fault in order to cache our whole 500MB file? That sounds like a deal a little too good to be true. We need to dig deeper and collect some more numbers. Enter [bpftrace](https://github.com/bpftrace/bpftrace).

### bpftrace

First of all, we can count the major faults with a bpftrace one-liner like so, just as a confirmation that we can trust what we're seeing:
```
sudo bpftrace -e 'software:major-faults { @[comm] = count(); }'
```
If we run our program again, sure enough, bpftrace gives us this output (I've recreated our test file in the meantime):
```
@[wc-go]: 1
```

Ok, great, but what we'd really like to do is get quantitative at this point and understand what kind of overhead `mmap` really incurs in our use case. With `bpftrace`, we can get a lot more insights. For example, this one-liner:
```
sudo bpftrace -e 'kprobe:handle_mm_fault { @ts[tid] = nsecs;} kretprobe:handle_mm_fault /@ts[tid]/ {@lat[comm] = hist(nsecs - @ts[tid]); delete(@ts[tid]);}'
```
can give us a histogram of times spent handling memory map faults. And this is the output it gives:
```
...
@lat[wc-go]: 
[256, 512)            30 |                                                    |
[512, 1K)           4432 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[1K, 2K)            3768 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@        |
[2K, 4K)             342 |@@@@                                                |
[4K, 8K)              30 |                                                    |
[8K, 16K)           2164 |@@@@@@@@@@@@@@@@@@@@@@@@@                           |
[16K, 32K)          1775 |@@@@@@@@@@@@@@@@@@@@                                |
[32K, 64K)           196 |@@                                                  |
[64K, 128K)           18 |                                                    |
[128K, 256K)           4 |                                                    |
[256K, 512K)           6 |                                                    |
[512K, 1M)             2 |                                                    |
[1M, 2M)               2 |                                                    |
[2M, 4M)               0 |                                                    |
[4M, 8M)               1 |                                                    |
...
```

we can see that the majority of the latencies are around the microsecond mark. I have an SSD in this machine, and we'd expect an I/O read to be in the tens to hundreds of microseconds. And some handler calls do take that long. That 4-8ms call looks like it must have done some heavy lifting.

Nevertheless, very few high latency fault handler invocations. Let's note that what we see here sums up to about 30ms. As it tunrs out, Linux has some more tricks up its sleeve that give us a boost here, namely a mechanism called "readahead". This detects that we are going through our file sequentially and loads more data than needed when it seems like we will be accessing it. Let's verify if this is indeed what is happening, by running:
```
sudo bpftrace -e 'kprobe:page_cache_async_ra { @[comm] = count(); }'
```
And sure enogh, the output is:
```
@[wc-go]: 4147
```
There it is, doing the work so we don't bump up against those pesky major faults. And if we run it again on the same file, we get no more readaheads, because, of course, the file is already cached as we saw before.

## System-level comparison: `mmap` vs `upfront`

We went rather deep on `mmap`, but what about the other versions of our code? Now that we've run quite a few kinds of tests, let's focus on the system-level view of 2 versions of our code: `upfront` and `mmap`. These were the 2 best performing versions under our test conditions: we're using a fairly large file, but clearly one that fits quite comfortably in memory. If the file were much larger, `mmap` would have more work to do (more higher latency operations involved) and `upfront` wouldn't be feasible, so we'd have to compare it against `scanner`. But for now, let's focus on this use case with `mmap` vs. `upfront`.

### perf tools

Let's start with the big picture:
```
sudo perf stat -d ./wc-go -p mmap ~/shakespeare100.txt
```

This gives us the following output:
```
Performance counter stats for './wc-go -p mmap ~/shakespeare100.txt':

          3,870.47 msec task-clock                       #    1.001 CPUs utilized             
               622      context-switches                 #  160.704 /sec                      
                32      cpu-migrations                   #    8.268 /sec                      
             8,619      page-faults                      #    2.227 K/sec                     
...
       3.865924351 seconds time elapsed

       3.726353000 seconds user
       0.148212000 seconds sys
```

and for the upfront version:
```
 Performance counter stats for './wc-go -p upfront ~/shakespeare100.txt':

          3,932.69 msec task-clock                       #    0.999 CPUs utilized             
               601      context-switches                 #  152.822 /sec                      
                20      cpu-migrations                   #    5.086 /sec                      
           133,378      page-faults                      #   33.915 K/sec                     
...
       3.937098892 seconds time elapsed

       3.630355000 seconds user
       0.306126000 seconds sys
```

What jumps out here is that the sys time in the upfront implementation is roughly double compared to `mmap`. This could have something to do with the fact that the `upfront` implementation also has to copy the data from page cache to user buffers, which costs CPU (memcpy in the kernel). We also see way more page-faults reported for the `upfront` version, which looks surprising. This might have something to do with how page-faults are counted here, as again, the `upfront` version can fault on both the file pages and the buffer pages.

We can run a `sudo perf record -g ./wc-go -p upfront ~/shakespeare100.txt` followed by `sudo perf report` (and similarly for `mmap`) and observe where exactly the upfront version spends more time in syscalls and we will indeed see vfs_read dominating there, but also quite a bit of time on *_copy_to_user.

Now let's have a deeper look into this comparison using bpftrace.

### bpftrace

First, we can observe the much larger number of minor page faults in the `upfront` version with this one-liner:
```
sudo bpftrace -e 'software:minor-faults { @[comm] = count(); }'
```

And recalling the histogram for handle_mm_fault latencies for the mmap version from before, let's run the same for the `upfront` version for comparison:
```
@lat[wc-go]: 
[128, 256)            17 |                                                    |
[256, 512)        109975 |@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@|
[512, 1K)          16412 |@@@@@@@                                             |
[1K, 2K)            2578 |@                                                   |
[2K, 4K)            3671 |@                                                   |
[4K, 8K)             399 |                                                    |
[8K, 16K)            126 |                                                    |
[16K, 32K)           163 |                                                    |
[32K, 64K)            39 |                                                    |
[64K, 128K)            2 |                                                    |
[128K, 256K)           2 |                                                    |
[256K, 512K)           0 |                                                    |
[512K, 1M)             0 |                                                    |
[1M, 2M)               0 |                                                    |
[2M, 4M)               0 |                                                    |
[4M, 8M)               0 |                                                    |
[8M, 16M)              0 |                                                    |
[16M, 32M)             1 |                                                    |
```

The difference is significant: many more page faults, but all grouped in the well-sub-microsecond area. These page faults are not the ones doing the heavy lifting in this implementation.


