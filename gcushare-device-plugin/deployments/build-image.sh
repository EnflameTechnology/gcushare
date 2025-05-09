#!/bin/bash
#
# Copyright (c) 2024 Enflame. All Rights Reserved.
#

if [ -f "./build-image.conf" ]; then
    source ./build-image.conf
else
    echo "build-image.conf is not found, exit."; exit 0
fi

function usage() {
  cat <<EOF
Usage: ./$0 [OPTIONS]...
Description: This script is used to build and save images.

Options:
  --os        Specify the operating system for the image. Currently supports "ubuntu, tlinux and openeuler", default: "$OS".
  --cli       Specify the CLI tool for building the image. Currently supports "docker, ctr, podman and nerdctl", default: "$CLI_NAME".
  --repo      Specify the repository for the image that will be built, default: "$REPO_NAME".
  --name      Specify the name of the image that will be built, default: "$IMAGE_NAME".
  --tag       Specify the tag for the image that will be built, default: "$TAG".
  --namespace Specify the namespace for the image that will be built by nerdctl and ctr, default: "$NAMESPACE".

Examples:
  ./$0
  ./$0 --cli podman --os ubuntu
  ./$0 --cli nerdctl --os openeuler
  ./$0 --cli ctr --os ubuntu
EOF
}

function printEnv() {
    echo -e "\033[31m################## config build-image.conf if need #####################\033[0m"
    echo -e "\033[33m OS=${OS} \n CLI_NAME=${CLI_NAME} \n REPO_NAME=${REPO_NAME}\033[0m"
    echo -e "\033[33m IMAGE_NAME=${IMAGE_NAME} \n TAG=${TAG} \n NAMESPACE=${NAMESPACE}\033[0m"
    echo -e "\033[31m########################################################################\033[0m"
}

function checkCommand() {
    if [ "$CLI_NAME" == "docker" ]; then
        if ! command -v docker > /dev/null 2>&1; then
            echo "docker is not found, exit."; exit 1
        fi
    elif [ "$CLI_NAME" == "podman" ]; then
        if ! command -v podman > /dev/null 2>&1; then
            echo "podman is not found, exit."; exit 1
        fi
    elif [ "$CLI_NAME" == "nerdctl" ]; then
        if ! command -v nerdctl > /dev/null 2>&1; then
            echo "nerdctl is not found, exit."; exit 1
        fi
    elif [ "$CLI_NAME" == "ctr" ]; then
        if ! command -v ctr > /dev/null 2>&1; then
            echo "ctr is not found, exit."; exit 1
        fi
        if ! command -v docker > /dev/null 2>&1; then
            echo "docker is not found, exit."; exit 1
        fi
    else
        echo "Invalid cli provided: $CLI_NAME"; exit 1
    fi
}

function clearImage() {
    echo -e "\033[33mClear old image if exist.\033[0m"
    if [ "$CLI_NAME" == "nerdctl" ]; then
        $CLI_NAME --namespace $NAMESPACE rmi -f $REPO_FULL_NAME > /dev/null 2>&1 || true
    elif [ "$CLI_NAME" == "ctr" ]; then
        $CLI_NAME --namespace $NAMESPACE images rm $REPO_FULL_NAME > /dev/null 2>&1 || true
    else
        $CLI_NAME rmi -f $REPO_FULL_NAME  > /dev/null 2>&1 || true
    fi
}

function buildImage() {
    echo -e "\033[33mBuilding image: $REPO_FULL_NAME start...\033[0m"
    if [ "$CLI_NAME" == "nerdctl" ]; then
        $CLI_NAME --namespace $NAMESPACE build -t $REPO_FULL_NAME --file dockerfiles/Dockerfile.$OS .
    elif [ "$CLI_NAME" == "ctr" ]; then
        docker build -t $REPO_FULL_NAME --file dockerfiles/Dockerfile.$OS .
    else
        $CLI_NAME build -t $REPO_FULL_NAME --file dockerfiles/Dockerfile.$OS .
    fi
    echo "The image built successfully."
}

function saveImage() {
    echo -e "\033[33mSave image package to ./images\033[0m"
    if [ -d "./images" ]; then
        rm -rf ./images
    fi
    mkdir -p ./images

    IMAGE_SAVED_NAME="${IMAGE_NAME}.tar"
    if [ "$CLI_NAME" == "nerdctl" ]; then
        $CLI_NAME --namespace $NAMESPACE save -o ./images/$IMAGE_SAVED_NAME $REPO_FULL_NAME
    elif [ "$CLI_NAME" == "ctr" ]; then
        docker save -o ./images/$IMAGE_SAVED_NAME $REPO_FULL_NAME
        $CLI_NAME --namespace $NAMESPACE image import ./images/$IMAGE_SAVED_NAME
    else
        $CLI_NAME save -o ./images/$IMAGE_SAVED_NAME $REPO_FULL_NAME
    fi
}

function listImage() {
    echo -e "\033[33mList images...\033[0m"
    if [ "$CLI_NAME" == "nerdctl" ]; then
        $CLI_NAME --namespace $NAMESPACE images | grep $IMAGE_NAME
    elif [ "$CLI_NAME" == "ctr" ]; then
        $CLI_NAME --namespace $NAMESPACE images list | grep $IMAGE_NAME
    else
        $CLI_NAME images | grep $IMAGE_NAME
    fi
}

while true :
    do
        [[ "$#" -lt 1 ]] && { break;}
        case $1 in
            "")
                break;;
            --os)
                OS=$2
                shift;;
            --cli)
                CLI_NAME=$2
                shift;;
            --repo)
                REPO_NAME=$2
                shift;;
            --name)
                IMAGE_NAME=$2
                shift;;
            --tag)
                TAG=$2
                shift;;
            --namespace)
                NAMESPACE=$2
                shift;;
            --help | -h)
                usage
                exit 0;;
            *)
                echo -e "\033[31mError! Unknown parameter: $1. Please enter the specified parameters according to the usage\033[0m"
                usage
                exit 1;;
        esac
    shift
done

REPO_FULL_NAME="${REPO_NAME}/${IMAGE_NAME}:${TAG}"

printEnv
checkCommand

clearImage
buildImage
saveImage
listImage
