## Time tests

Let's start with the very basics: just about the simplest test we can run - timing the process run. I will use a file called `shakespeare100.txt`, which is formed by concatenating a file containing the complete works of Shakespeare 100 times over (this results in a file just over 500MB in size - I did not check it in, obviously). A shakespeare.txt file can easily be found online and concatenating it can be done like so:

```
for i in {1..100};do cat shakespeare.txt >> shakespeare100.txt; done
```

So, running the times, first for the standard `wc` program:
```
% time wc shakespeare100.txt 

 12418500  89958800 543647500 shakespeare100.txt

real    0m1.634s
user    0m1.566s
sys     0m0.065s
```

And now for the different versions of our own program:

`scanner` processor:
```
% time ./wc-go -p scanner shakespeare100.txt 
12418500        89958800        531229000       shakespeare100.txt

real    0m2.797s
user    0m2.754s
sys     0m0.220s
```

`upfront` processor:
```
% time ./wc-go -p upfront shakespeare100.txt 
12418500        89958800        531229000       shakespeare100.txt

real    0m2.065s
user    0m1.877s
sys     0m0.194s
```

`buffering` processor:
```
% time ./wc-go -p buffering shakespeare100.txt 
12418500        89958800        531229000       shakespeare100.txt

real    0m7.030s
user    0m5.565s
sys     0m6.441s
```

`mmap` processor:
```
% time ./wc-go -p mmap shakespeare100.txt 
12418500        89958800        531229000       shakespeare100.txt

real    0m1.976s
user    0m1.942s
sys     0m0.031s
```

`mmap2` processor:
```
% time ./wc-go -p mmap2 shakespeare100.txt 
12418500        89958800        531229000       shakespeare100.txt

real    0m2.942s
user    0m2.910s
sys     0m0.199s
```
