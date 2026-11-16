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

function checkDRSPodExist() {
    drs_pod=""
    while read -r name ns annotation; do
        if [[ "$annotation" == *instanceID* ]]; then
            instanceID=$(echo "$annotation" | sed -E 's/.*"instanceID":"([^"]+)".*/\1/')
            if [[ -n "$instanceID" ]]; then
                drs_pod="$drs_pod\nname: $name, namespace: $ns"
            fi
        fi
    done < <(kubectl get pods -A -o jsonpath="{range .items[*]}{.metadata.name} {.metadata.namespace} {.metadata.annotations.assigned-containers}{'\n'}{end}")
    if [[ -n "$drs_pod" ]]; then
        echo -e "\033[33mWarning!!! The following pods are found to be using drs instance:\033[0m"
        echo -e $drs_pod
        echo -e "\033[33mIf you delete these pods after the gcushare device plugin is uninstalled,"\
        "the corresponding drs will not be automatically cleaned up.\033[0m"
        echo -e "\033[33mIt is recommended to delete these pods before uninstalling the gcushare device plugin,"\
        "otherwise the remaining drs cannot be used by gcushare again until you manually delete these remaining drs.\033[0m"
        echo -e "\033[33mContinue uninstalling gcushare device plugin?(y/yes, n/no)\033[0m"
        read -e input
        case $input in
        (y | yes)
            ;;
        (n | no | *)
            echo -e "\033[33mExit\033[0m"
            exit 0;;
        esac
    fi
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

checkDRSPodExist

if ! command -v helm >/dev/null 2>&1; then
    kubectl delete -f yaml/gcushare-device-plugin.yaml
else
    printf "Uninstall gcushare device plugin release in namespace:%s start...\n" $NAMESPACE
    helm uninstall $RELEASE_NAME -n $NAMESPACE
fi
unlabelClusterGCUShareNodes
