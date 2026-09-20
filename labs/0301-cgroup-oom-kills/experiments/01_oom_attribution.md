## OOM attribution: ig vs Docker's own signals

Tests Predictions 1, 2, and 3 together - one clean run of both containers exercises all three: which process actually gets killed (Prediction 2), whether the container survives it (Prediction 3), and whether `ig`'s trace gives a materially different picture than Docker's own `OOMKilled`/`Status` fields (Prediction 1).

Rebuild and start both containers fresh:
```sh
docker compose -f compose.yml up -d --build --force-recreate
```

Trace both kills as they happen. Both containers should hit their limit and get a process killed within the first several seconds, so a 20s window is comfortably enough:
```sh
docker exec lab-analysis timeout -s INT 20 ig run trace_oomkill:latest > /tmp/oom_trace.txt 2>&1 &
sleep 20
cat /tmp/oom_trace.txt
```
producing output:
```
RUNTIME.CONTAINERNAME          COMM                          PID              TID TID                          TPID TCOMM            PAGES           
lab-oom-multi                  hog                         19846            19846 19775                       19846 hog              51200           
lab-oom-single                 hog                         19811            19811 19790                       19811 hog              51200  
```

Compare against what Docker itself reports for the same two containers - this is the direct comparison for Prediction 1:
```sh
docker inspect lab-oom-single --format '{{.State.Status}} exitcode={{.State.ExitCode}} OOMKilled={{.State.OOMKilled}}'
docker inspect lab-oom-multi  --format '{{.State.Status}} exitcode={{.State.ExitCode}} OOMKilled={{.State.OOMKilled}}'
```
producing output:
```
exited exitcode=137 OOMKilled=true
running exitcode=0 OOMKilled=true
```

Check which of the two processes in `lab-oom-multi` is still alive - the direct evidence for Prediction 2 (which one got picked) and Prediction 3 (why the container didn't stop):
```sh
docker logs lab-oom-multi | tail -10
small holding 20 MB
small holding 20 MB
small holding 20 MB
small holding 20 MB
small holding 20 MB
small holding 20 MB
small holding 20 MB
small holding 20 MB
small holding 20 MB
small holding 20 MB
```

Clean up when done:
```sh
docker compose -f compose.yml down
```
