# wc-go

This is a simplified version of the standard Unix command-line utility [wc](https://man7.org/linux/man-pages/man1/wc.1.html). It doesn't support all the flags and arguments. In fact, it will always just print out the number of lines, number of words, and number of characters in the input. The input can be a file, or stdin in case no file path is provided as an argument.

Instead, though, it does support some other arguments (the `-p` flag), which indicate strategies for how we will process the input. (for our tests). The strategies we are testing are:
- `scanner` - creates a bufio.Scanner directly on the os.File (or stdin) to process the input line by line and allocate and return a string per line to be processed in successive calls to scanner.Text. This would be the most common, idiomatic approach that you would see in the wild - and probably appropriate in most cases.
- `upfront` - calls io.ReadAll or os.ReadFile depending on the case - loading the whole contents in memory before processing.
- `buffering` - manually fill the buffer that eventually contains all the data, loaded in chunks, and then do the processing.
- `mmap` - (only works with files) use the mmap system call to get read only access to the file data as a byte slice and then do the processing on it.
- `mmap2` - same as mmap, but we will process the file data using a scanner, just to see the effect.

In all strategies except for the `scanner` and `mmap2` ones, once the data is available as a byte slice, we do all processing directly on the bytes and avoid any extra heap allocations.
