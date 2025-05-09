#!/bin/bash
#
# Copyright 2023 Enflame. All Rights Reserved.
#

set -eu -o pipefail

PKG_VER=${GCUSHARE_VERSION:-"0.0.0"}

function usage() {
  cat <<EOF
Usage: ./build.sh [command]
Command:
    all       Build the gcushare-scheduler-plugin
    clean     Clean the gcushare-scheduler-plugin
Example:
    ./build.sh all
    ./build.sh clean
EOF
}


function build_all() {
    if [ -d "./dist" ]; then
        clean_all
    fi
    echo -e "\033[33mBuilding gcushare-scheduler-plugin package start...\033[0m"

    PKG_DIR="dist/gcushare-scheduler-plugin_${PKG_VER}"
    mkdir -p $PKG_DIR/binarys
    cd $PKG_DIR/binarys
    mkdir v1 v1beta1
    cd -
    cp -rf -L deployments/* ${PKG_DIR}

    cd src/v1beta1
    go build -ldflags="-X main.Version=${PKG_VER}" -o ../../${PKG_DIR}/binarys/v1beta1/gcushare-scheduler-plugin main.go
    cd -

    cd src/v1
    go build -ldflags="-X main.Version=${PKG_VER}" -o ../../${PKG_DIR}/binarys/v1/gcushare-scheduler-plugin main.go
    cd -

    mkdir -p ${PKG_DIR}/config
    cp -rf ../config/topscloud.json ${PKG_DIR}/config/

    echo -e "\033[33mBuilding gcushare-scheduler-plugin package successfully!\033[0m"
}

function clean_all() {
    if [ -d "./dist" ]; then
        rm -rf ./dist
    fi
    echo -e "\033[33mClean gcushare-scheduler-plugin package successfully.\033[0m"
}

function main() {
    [[ "$EUID" -ne 0 ]] && { echo "[INFO] Run as non-root"; }
    [[ "$#" -lt 1 ]] && { usage >&2; exit 1; }

    case $1 in
    "clean")
        clean_all;;
    "all")
        build_all;;
    *)
        usage
        exit 1;;
    esac
}

main "$@"
