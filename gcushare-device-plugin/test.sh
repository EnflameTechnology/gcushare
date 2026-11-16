#!/bin/bash
#
# Copyright 2024 Enflame. All Rights Reserved.
#

# [[ "$EUID" -ne 0 ]] && { echo -e "\033[31mError: Test cases must be run as root\033[0m"; exit 1; }

# It is recommended to run this script before submitting the code to ensure that all test cases pass
export LOGPATH="$PWD/test.log"
# go test -v gcushare-device-plugin -gcflags=all=-l -cover
go test -v ./... -gcflags=all=-l -cover