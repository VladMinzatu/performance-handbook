#!/bin/bash

N=5  # number of producers
IPC_TYPE=unixsock
MSG_SIZE=100

for i in $(seq 1 $N); do
    echo "Starting producer $i..."
    go run cmd/producer/main.go "$IPC_TYPE" "$MSG_SIZE" &
done

wait
echo "All instances finished."

