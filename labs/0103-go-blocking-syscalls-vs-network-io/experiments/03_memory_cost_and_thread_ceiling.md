## Memory cost and the 10,000-thread ceiling

We test here our 3rd hypothesis: `MODE=syscall` should carry a materially higher memory footprint than `MODE=timer`/`MODE=netpoll` at the same `WORKERS`, and should reach Go's hard-coded 10000-OS-thread-per-process limit - something the other two modes can't structurally hit.

With the lab's default setup running (`docker compose -f compose.yml up -d --build`) we can run:
```sh
docker stats --no-stream lab-go-worker-timer lab-go-worker-syscall lab-go-worker-netpoll

CONTAINER ID   NAME                    CPU %     MEM USAGE / LIMIT     MEM %     NET I/O           BLOCK I/O     PIDS
c951c8340806   lab-go-worker-timer     3.11%     8.082MiB / 11.74GiB   0.07%     1.25kB / 126B     1.99MB / 0B   11
ca8224bc0620   lab-go-worker-syscall   10.65%    71.5MiB / 11.74GiB    0.60%     1.21kB / 126B     2.79MB / 0B   2010
6751c95514ea   lab-go-worker-netpoll   32.56%    23.7MiB / 11.74GiB    0.20%     33.7MB / 17.4MB   4.1kB / 0B    19
```

Cross-check with `/proc/1/status`'s `VmRSS` directly:
```sh
echo "timer:";   docker exec lab-go-worker-timer   sh -c 'grep VmRSS /proc/1/status'
timer:
VmRSS:	    9832 kB

echo "syscall:"; docker exec lab-go-worker-syscall sh -c 'grep VmRSS /proc/1/status'
syscall:
VmRSS:	   75152 kB

echo "netpoll:"; docker exec lab-go-worker-netpoll  sh -c 'grep VmRSS /proc/1/status'
netpoll:
VmRSS:	   25468 kB
```

`WORK_MS=200`'s thread reuse (see previous experiment) means thread count doesn't track `WORKERS` 1:1 once `WORKERS` gets large - so to force genuinely simultaneous blocking, use `WORK_MS=60000` with `WORKERS` comfortably past 10,000:
```sh
docker stop lab-go-worker-syscall
docker rm lab-go-worker-syscall

docker run -d --name lab-go-worker-syscall --network labnet \
  -e MODE=syscall -e WORKERS=12000 -e WORK_MS=60000 \
  0103-go-blocking-syscalls-vs-network-io-worker-syscall:latest
sleep 20
docker logs lab-go-worker-syscall
docker ps -a --filter name=lab-go-worker-syscall --format '{{.Names}}\t{{.Status}}'
```

and that produces:
```sh
goroutine 11844 gp=0x40186b5c00 m=nil [runnable]:
main.main.func1()
	/src/worker.go:94 fp=0x40186b87d0 sp=0x40186b87d0 pc=0xe8cf0
runtime.goexit({})
	/usr/local/go/src/runtime/asm_arm64.s:1223 +0x4 fp=0x40186b87d0 sp=0x40186b87d0 pc=0x82ca4
created by main.main in goroutine 1
	/src/worker.go:94 +0x384

goroutine 11845 gp=0x40186b5dc0 m=9901 mp=0x4018432708 [syscall]:
syscall.Syscall(0x65, 0x40186b8f80, 0x0, 0x0)
	/usr/local/go/src/syscall/syscall_linux.go:73 +0x20 fp=0x40186b8f20 sp=0x40186b8ec0 pc=0x9f8d0
syscall.Nanosleep(0x0?, 0x0?)
	/usr/local/go/src/syscall/zsyscall_linux_arm64.go:690 +0x34 fp=0x40186b8f60 sp=0x40186b8f20 pc=0x9e3b4
main.syscallWorker(0x0?)
	/src/worker.go:40 +0x84 fp=0x40186b8fa0 sp=0x40186b8f60 pc=0xe85c4
main.main.func1()
	/src/worker.go:97 +0xa4 fp=0x40186b8fd0 sp=0x40186b8fa0 pc=0xe8d94
runtime.goexit({})
	/usr/local/go/src/runtime/asm_arm64.s:1223 +0x4 fp=0x40186b8fd0 sp=0x40186b8fd0 pc=0x82ca4
created by main.main in goroutine 1
	/src/worker.go:94 +0x384
lab-go-worker-syscall	Exited (2) 20 seconds ago
```

Just to cross check: will the other workers survive with the same settings? Let's try netpoll:
```sh
docker stop lab-go-worker-netpoll
docker rm lab-go-worker-netpoll

docker run -d --name lab-go-worker-netpoll --network labnet \
  -e MODE=netpoll -e WORKERS=12000 -e WORK_MS=60000 -e DELAY_SERVER_ADDR=delay-server:9100 \
  0103-go-blocking-syscalls-vs-network-io-worker-netpoll:latest
sleep 15
docker logs lab-go-worker-netpoll | tail -5
docker ps -a --filter name=lab-go-worker-netpoll --format '{{.Names}}\t{{.Status}}'
docker exec lab-go-worker-netpoll sh -c 'grep Threads /proc/1/status'
```

Of course it does. This produces the output:
```sh
goroutines=12001
goroutines=12001
goroutines=12001
goroutines=12001
goroutines=12001
lab-go-worker-netpoll	Up 15 seconds
Threads:	27
```

Let's also try the timer, just for completeness:
```sh
docker stop lab-go-worker-timer
docker rm lab-go-worker-timer

docker run -d --name lab-go-worker-timer --network labnet \
  -e MODE=timer -e WORKERS=12000 -e WORK_MS=60000 \
  0103-go-blocking-syscalls-vs-network-io-worker-netpoll:latest
sleep 15
docker logs lab-go-worker-timer | tail -5
docker ps -a --filter name=lab-go-worker-timer --format '{{.Names}}\t{{.Status}}'
docker exec lab-go-worker-timer sh -c 'grep Threads /proc/1/status'
```
which produces the output:
```sh
goroutines=12001
goroutines=12001
goroutines=12001
goroutines=12001
goroutines=12001
lab-go-worker-timer	Up 15 seconds
Threads:	9
```
Easy!
