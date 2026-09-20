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

Both processes land at essentially the same resident size (51200 pages ≈ 200MB) before being killed - expected, since the two hogs are structurally identical and only `CAP_MB` differs. The two rows are only distinguishable by container name and PID here - both show `comm=hog` - which is exactly the attribution `docker logs`/`ps` alone can't give: without `ig`, there'd be no way to tell these two kills apart at all, let alone confirm each one's actual size at the moment of death.

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

Confirms Predictions 1 and 3 directly. `lab-oom-single` had only one process, so killing it ends the container outright - `exited`, `137`, `OOMKilled=true`, the intuitive case. `lab-oom-multi` shows `running` and `OOMKilled=true` *simultaneously* - Docker recorded a real OOM kill against this container, yet it never stopped. Read on its own, `OOMKilled: true` on a container that's still running looks like a contradiction; it isn't, once the kill is known to have landed on a background process rather than the container's own PID 1.

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

Only `small` is still logging - `big` is gone. Confirms Prediction 2: between two identically-named processes sharing one memory limit, the OOM killer picked the one actually holding more resident memory, not the one that started first or any other detail of process identity. It also closes the loop on the previous step: `big` was the container's non-essential background process, so losing it left the launcher script's `wait` - and the container - running, which is why `lab-oom-multi` stayed up despite the kill.

Clean up when done:
```sh
docker compose -f compose.yml down
```
