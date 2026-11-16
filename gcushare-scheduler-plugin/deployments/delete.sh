#!/bin/bash
#
# Copyright (c) 2022 Enflame. All Rights Reserved.
#

RELEASE_NAME="gcushare-scheduler-plugin"
NAMESPACE="kube-system"

function usage() {
  cat <<EOF
Usage: ./delete.sh <param name> <param value>
Description: This script is used to delete gcushare scheduler plugin component
Param:
    --name        The release name of gcushare scheduler plugin, optional, default gcushare-scheduler-plugin.
    --namespace   The release namespace of gcushare scheduler plugin, optional, default kube-system.
Example:
    ./delete.sh --namespace kube-system
EOF
}

function version_ge() { test "$(echo "$@" | tr " " "\n" | sort -rV | head -n 1)" == "$1"; }

function pluginUnInstall() {
    if ! command -v helm >/dev/null 2>&1; then
        k8sVersion=$(kubectl get node -o jsonpath='{.items[0].status.nodeInfo.kubeletVersion}')
        baseVersion="v1.24.0"
        if version_ge $k8sVersion $baseVersion; then
            kubectl delete -f yaml/v1/gcushare-scheduler-plugin.yaml
        else
            kubectl delete -f yaml/v1beta1/gcushare-scheduler-plugin.yaml
        fi
    else
        printf "Uninstall gcushare scheduler plugin release in namespace:%s start...\n" $NAMESPACE
        helm uninstall $RELEASE_NAME -n $NAMESPACE
    fi
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

pluginUnInstall