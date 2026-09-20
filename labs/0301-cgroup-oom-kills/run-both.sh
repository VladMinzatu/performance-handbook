#!/bin/sh
# Launches two hog instances in one container, sharing one cgroup memory
# limit: NAME=small stays capped well under the limit, NAME=big grows
# without bound - see README Prediction 2 (which one gets picked as OOM
# victim) and Prediction 3 (whether the container survives losing it).
NAME=small CAP_MB=20 hog &
NAME=big CAP_MB=0 hog &
wait
