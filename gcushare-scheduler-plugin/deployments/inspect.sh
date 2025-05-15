#!/bin/bash
# Copyright (c) 2025, ENFLAME INC.  All rights reserved.

nodeName=""
drs="false"

function usage() {
  cat <<EOF
Usage: ./inspect.sh <param name> <param value>
Description: This script is used to display the usage of GCU devices.
Param:
    --node      Optional. Query the usage of the GCU device of the specified node, by default, all nodes are queried.
    --drs       Optional. Whether to query the usage of drs GCU, default 'false'
Example:
    ./inspect.sh --node node1 --drs true
EOF
}

function main() {
    serviceName="gcushare-scheduler-plugin"
    serviceIp=`kubectl get svc -A | grep $serviceName | awk '{print $4}'&`
    port="32766"
    
    url="$serviceIp:$port/gcushare-scheduler/inspect"
    if [[ $nodeName != "" ]]; then
        url="$url/$nodeName"
    fi
    curl $url?drs=$drs
}



while true :
    do
        [[ "$#" -lt 1 ]] && { break;}
        case $1 in
            "")
                break;;
            --node)
                nodeName=$2
                shift;;
            --drs)
                drs=$2
                shift;;
            --help | -h)
                usage
                exit 0;;
            *)
                echo "Usage error! Please refer the usage to execute the command"
                usage
                exit 1;;
        esac
    shift
done

main