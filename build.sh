#!/bin/bash
#
# Copyright 2024 Enflame. All Rights Reserved.
#


export GCUSHARE_VERSION="1.0.1"

PKG_VER=${GCUSHARE_VERSION:-"0.0.0"}

function usage() {
  cat <<EOF
Usage: ./build.sh [command]
Command:
    all       Build the gcushare
    clean     Clean the gcushare
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
    echo -e "\033[33mBuilding gcushare package start...\033[0m"

    PKG_DIR="dist/gcushare_$PKG_VER"
    mkdir -p $PKG_DIR

    cd gcushare-device-plugin
    ./build.sh all
    cp -rf dist/* ../$PKG_DIR/
    rm -rf dist
    cd -

    cd gcushare-scheduler-plugin
    ./build.sh all
    cp -rf dist/* ../$PKG_DIR/
    rm -rf dist
    cd -

    echo -e "\033[33mBuilding gcushare package successfully!\033[0m"
}

function clean_all() {
    if [ -d "./dist" ]; then
        rm -rf ./dist
    fi
    cd gcushare-device-plugin
    ./build.sh clean
    cd -
    cd gcushare-scheduler-plugin
    ./build.sh clean
    cd -
    echo -e "\033[33mClean gcushare package successfully.\033[0m"
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
