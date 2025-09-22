#!/bin/bash

N=5  # number of producers
IPC_TYPE=udp
MSG_SIZE=100

for i in $(seq 1 $N); do
    echo "Starting producer $i..."
    ./cmd/producer/producer "$IPC_TYPE" "$MSG_SIZE" &
done

wait
echo "All instances finished."

