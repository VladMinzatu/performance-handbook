## GOMAXPROCS

With both containers running as explained in the setup, we can run:
```sh
docker logs lab-go-cpuburn-default | head -1
GOMAXPROCS=8 NumCPU=8 numWorkers=8
```
and
```sh
docker logs lab-go-cpuburn-fixed | head -1
GOMAXPROCS=2 NumCPU=8 numWorkers=2
```

And thus confirm that the GOMAXPROX setting will be equal to the number of CPUs regardless of the  cgroup CPU quotas.

`cpuburn-fixed`'s line only shows `GOMAXPROCS=2` because of the envitonment variable.
