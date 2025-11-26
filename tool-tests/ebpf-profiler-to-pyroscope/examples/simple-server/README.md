# simple-server

Simple server implementation that uses pyroscope's in-process profiling, which uses runtime APIs and hooks, but in a lightweight, sampled way.

Requires pyroscope to be running locally as per instructions in the [parent directory](../..).

To generate some traffic in a simple way, we can run the following:
```
while true; do curl -s http://localhost:8080/fast >/dev/null; curl -s http://localhost:8080/mixed >/dev/null; curl -s http://localhost:8080/slow >/dev/null; sleep 0.1; done
```

Then we can check what is collected in pyroscope at `localhost:4040`.