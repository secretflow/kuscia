#!/bin/bash
ls
nohup prometheus --config.file=/home/config/prometheus.yml > /dev/null 2>&1 &
nohup grafana server > /dev/null 2>&1 &
trap "exit 0" SIGTERM
while true; do sleep 1; done
