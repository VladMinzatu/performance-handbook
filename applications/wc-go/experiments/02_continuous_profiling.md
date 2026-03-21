## Continuous Profiling

We will now look at the continuous profiling outputs for the different file processing strategies. We will use Parca and its agent this time. To start up the profilign infra, run:
```
docker compose up
```

And then we will be running our binary in a loop like so:
```
for i in {1..100}; do
  ./wc-go -p scanner shakespeare1000.txt
done
```

Note that we are using bigger files here (1000 x shakespeare) because for short lived processes, symbolization data is not yet available. So the runs produce the following outcomes:

`scanner`:
![Parca scanner](./assets/parca-wc-scanner.png)

`upfront`
![Parca upfront](./assets/parca-wc-upfront.png)

`buffering`
![Parca buffering](./assets/parca-wc-buffering.png)

`mmap`
![Parca mmap](./assets/parca-wc-mmap.png)

`mmap2`
![Parca mmap2](./assets/parca-wc-mmap2.png)
