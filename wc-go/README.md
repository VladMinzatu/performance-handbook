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
