#!/bin/sh
# Native build:            ./build.sh
# Windows exe from Linux:  GOOS=windows GOARCH=amd64 ./build.sh
set -e
go mod tidy
out=mcserver; [ "$GOOS" = "windows" ] && out=mcserver.exe
go build -ldflags "-s -w" -o "$out" .
echo "Built $out"
