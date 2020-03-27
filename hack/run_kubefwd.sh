#!/bin/bash

# This script will run kubefwd in an infinite loop, restarting every 20 seconds in order to forward 
# the latest pods and services.

# Kubefwd can be installed from https://github.com/txn2/kubefwd

which kubefwd >/dev/null || { echo "Please install kubefwd"; exit 1; }

while true; do
    # Since kubefwd updates /etc/hosts file it has to be ran as sudo
    sudo kubefwd svc & q_pid="${!}"
    ( sleep 20 ; sudo kill "${q_pid}" ) & s_pid="${!}"
    wait "${q_pid}"
    sudo kill "${s_pid}"
    wait "${s_pid}"
done
