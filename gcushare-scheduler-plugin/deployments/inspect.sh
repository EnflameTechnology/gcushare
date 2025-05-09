#!/bin/bash
# Copyright (c) 2025, ENFLAME INC.  All rights reserved.

serviceName="gcushare-scheduler-plugin"
serviceIp=`kubectl get svc -A | grep $serviceName | awk '{print $4}'&`
port="32766"
if [ "$#" -ge 1 ]; then
    curl $serviceIp:$port/gcushare-scheduler/inspect/$1
else
    curl $serviceIp:$port/gcushare-scheduler/inspect
fi
