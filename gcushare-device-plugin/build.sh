#!/bin/bash
#
# Copyright 2023 Enflame. All Rights Reserved.
#

if [ -f "../../../g_version.txt" ]; then
    source ../../../g_version.txt
fi
PKG_VER=${GCUSHARE_VERSION:-"0.0.0"}

function usage() {
  cat <<EOF
Usage: ./build.sh [command]
Command:
    all       Build the gcushare-device-plugin
    clean     Clean the gcushare-device-plugin
Example:
    ./build.sh all
    ./build.sh clean
EOF
}

function build_all() {
    # clean old dist if exist
    if [ -d "./dist" ]; then
        clean_all
    fi
    echo -e "\033[33mBuilding gcushare-device-plugin package start...\033[0m"

    PKG_DIR="dist/gcushare-device-plugin_${PKG_VER}"
    mkdir -p $PKG_DIR
    go build -ldflags="-s -w -X main.Version=${PKG_VER}" -o ${PKG_DIR}/gcushare-device-plugin main.go

    cp -rf -L deployments/* ${PKG_DIR}

    mkdir -p ${PKG_DIR}/config
    cp -rf ../../common/topscloud.json ${PKG_DIR}/config/

    echo -e "\033[33mBuilding gcushare-device-plugin package successfully!\033[0m"
}

function clean_all() {
    if [ -d "./dist" ]; then
        rm -rf ./dist
    fi
    echo -e "\033[33mClean gcushare-device-plugin package successfully.\033[0m"
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
