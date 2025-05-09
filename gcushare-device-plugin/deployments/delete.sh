#!/bin/bash
# Copyright (c) 2022 Enflame. All Rights Reserved.

RELEASE_NAME="gcushare-device-plugin"
NAMESPACE="kube-system"

function usage() {
  cat <<EOF
Usage: ./delete.sh <param name> <param value>
Description: This script is used to delete gcushare device plugin component
Param:
    --name        The release name of gcushare device plugin, optional, default gcushare-device-plugin.
    --namespace   The release namespace of gcushare device plugin, optional, default kube-system.
Example:
    ./delete.sh --namespace kube-system
EOF
}

function unlabelClusterGCUShareNodes() {
    echo "Delete label enflame.com/gcushare=true of all cluster nodes..."
    nodes=`kubectl get nodes |grep -v NAME | awk '{print $1}'`
    kubectl label node ${nodes} enflame.com/gcushare-
}

while true :
    do
        case $1 in
            "")
                break;;
            --name)
                RELEASE_NAME=$2
                shift;;
            --namespace)
                NAMESPACE=$2
                shift;;
            --help)
                usage
                exit 0;;
            *)
                echo "Usage error! Please refer the usage to execute the command"
                usage
                exit 1;;
        esac
    shift
done

if ! command -v helm >/dev/null 2>&1; then
    kubectl delete -f yaml/gcushare-device-plugin.yaml
else
    printf "Uninstall gcushare device plugin release in namespace:%s start...\n" $NAMESPACE
    helm uninstall $RELEASE_NAME -n $NAMESPACE
fi
unlabelClusterGCUShareNodes
