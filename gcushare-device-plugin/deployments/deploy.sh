#!/bin/bash
# Copyright (c) 2022 Enflame. All Rights Reserved.

RELEASE_NAME="gcushare-device-plugin"
NAMESPACE="kube-system"

function usage() {
  cat <<EOF
Usage: ./deploy.sh <param name> <param value>
Description: This script is used to deploy gcushare device plugin component
Param:
    --name        The release name of gcushare device plugin, optional, default gcushare-device-plugin.
    --namespace   The release namespace of gcushare device plugin, optional, default kube-system.
Example:
    ./deploy.sh --namespace kube-system
EOF
}

function labelClusterGCUShareNodes() {
    nodes=`kubectl get nodes |grep -v NAME | awk '{print $1}'`
    echo "Cluster node name list:"
    echo -e "\033[33m[$nodes]\033[0m"
    echo "Please input gcushare node name and separated by space(if all nodes use shared GCU, you can just press Enter):"
    read -e input
    if [[ -z $input ]]; then 
        kubectl label nodes ${nodes} enflame.com/gcushare=true --overwrite=true
    else
        kubectl label nodes ${input} enflame.com/gcushare=true --overwrite=true
    fi
}

function pluginInstall() {
    if ! command -v helm >/dev/null 2>&1; then
        echo -e "\033[33mThe command helm not found, will deploy gcushare-device-plugin directly using kubectl\033[0m"
        kubectl create -f yaml/gcushare-device-plugin.yaml
        return
    fi
    echo "Deploy gcushare-device-plugin release start..."
    CHART_NAME=${RELEASE_NAME}-chart
    helm install $RELEASE_NAME $CHART_NAME -n $NAMESPACE
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

labelClusterGCUShareNodes
pluginInstall
