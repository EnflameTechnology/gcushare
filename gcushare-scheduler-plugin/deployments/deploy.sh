#!/bin/bash
#
# Copyright (c) 2022 Enflame. All Rights Reserved.
#

RELEASE_NAME="gcushare-scheduler-plugin"
NAMESPACE="kube-system"

function usage() {
  cat <<EOF
Usage: ./deploy.sh <param name> <param value>
Description: This script is used to deploy gcushare scheduler plugin component
Param:
    --name        The release name of gcushare scheduler plugin, optional, default gcushare-device-plugin.
    --namespace   The namespace of the gcushare-scheduler-plugin, optional, default kube-system.
Example:
    ./deploy.sh --namespace kube-system
EOF
}

function version_ge() { test "$(echo "$@" | tr " " "\n" | sort -rV | head -n 1)" == "$1"; }

function pluginInstall() {
    if ! command -v helm >/dev/null 2>&1; then
        echo -e "\033[33mThe command helm not found, will deploy gcushare-scheduler-plugin directly using kubectl\033[0m"
        k8sVersion=$(kubectl get node -o jsonpath='{.items[0].status.nodeInfo.kubeletVersion}')
        baseVersion="v1.24.0"
        if version_ge $k8sVersion $baseVersion; then
            kubectl create -f yaml/v1/gcushare-scheduler-plugin.yaml
        else
            kubectl create -f yaml/v1beta1/gcushare-scheduler-plugin.yaml
        fi
        return
    fi
    echo "Deploy gcushare-scheduler-plugin release start..."
    CHART_NAME=${RELEASE_NAME}-chart
    helm install $RELEASE_NAME $CHART_NAME -n $NAMESPACE
}

while true :
    do
        [[ "$#" -lt 1 ]] && { break;}
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

pluginInstall
