# pprof Block

With the pprof endpoint enabled and block profiling rate set to 1, we can use the Go tool to fetch the block profile data:
```
go tool pprof http://localhost:6060/debug/pprof/block
```

or alternatively:
```
curl http://localhost:6060/debug/pprof/block > block.prof
go tool pprof block.prof
```

Running `top` in the tool reveals the following:
```
(pprof) top
Showing nodes accounting for 664.11s, 100% of 664.11s total
Dropped 22 nodes (cum <= 3.32s)
Showing top 10 nodes out of 18
      flat  flat%   sum%        cum   cum%
   618.96s 93.20% 93.20%    618.96s 93.20%  runtime.selectgo
    21.88s  3.30% 96.50%     21.88s  3.30%  sync.(*RWMutex).RLock (inline)
    12.35s  1.86% 98.36%     12.35s  1.86%  runtime.chanrecv2
     9.39s  1.41% 99.77%      9.39s  1.41%  sync.(*Mutex).Lock (inline)
     1.53s  0.23%   100%     10.92s  1.64%  sync.(*RWMutex).Lock
```

So `select` is the top blocking site and it's not even close. But of course, each of our stages has 2 selects. Can we get anything more specific than this?

Recall that we started with all stages configured identically, with 10 workers and buffered channels with buffers of size 100 between them.

We can generate a block web by typing e.g. `png` to generate a png output:

![block web](./assets/block_profile.png)

The key thing to note here is that all stages block roughly equally, which is actually a good thing. It means that the pipeline is balanced and synchronized at its throughput limit.

To illustrate this, let's run an experiment: setting the number of workers of one of the stages (say, the `embed` step) to 1 (i.e. 1/10 the other stages).

![block web embed 1](./assets/block_profile_embed_1.png)

Now the `embed` stage is spending less time than the others blocked in `select`. But what is happening overall? Let's check our dashboard:

![grafana with embed 1](./assets/embed_1_grafana.png)

If anything, this run looks more stable run and the "underpowered" embed step did not become a bottleneck. It just spends less time blocked compared to the other stages.

This makes sense if we consider that we are so CPU bound throughout the pipeline. Is the time spent blocked in `select`s an indication of the overhead we introduce with our excessive number of workers given our CPU-bound pipeline?

Let's see what happens with just 2 worker per stage (since we have 2 CPU units for our container):

![2 workers grafana](./assets/2_workers_grafana.png)

Perhaps we've managed to eke out a slight throughput improvement. Let's see what the block profile looks like in this case.

![2 workers block](./assets/2_workers_block.png)

It looks like we're back to balance in our pipeline, while having cut some of the overall block time.

What we've seen in this section is that we could spot some unnecessary overhead that we could eliminate. (we cut the number of workers by a factor of 5 without any loss in throughput).

But it hasn't brought us spectacular gains here. If we had some IO heavy stages sprinkled in there, we probably could have made some interesting changes, like put more workers in the IO stage to get some real visible improvements.
