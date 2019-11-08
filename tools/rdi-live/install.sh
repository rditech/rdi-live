#!/bin/bash

# set up options
print_usage() {
    printf 'Usage: '
    printf "$0"
    printf ' [OPTIONS]\n'
    printf '\n'
    printf 'This tool is for installing live display software onto the local machine.\n'
    printf '\n'
    printf 'OPTIONS:\n'
}

while getopts 'h' flag; do
    case "${flag}" in
        *) print_usage
           exit 1 ;;
        esac
done

echo "stopping service..."
systemctl stop rdi-live

echo "building the executable..."
GOARCH=amd64 GOOS=linux go build
if [ "$?" != "0" ]; then
    exit 1
fi

bin=/usr/local/bin
echo "creating $bin..."
mkdir -p $bin
if [ "$?" != "0" ]; then
    exit 1
fi

echo "copying executable..."
cp rdi-live $bin/
if [ "$?" != "0" ]; then
    exit 1
fi

echo "installing rdi-live service..."
cp rdi-live.service /etc/systemd/system/rdi-live.service
if [ "$?" != "0" ]; then
    exit 1
fi
systemctl daemon-reload
if [ "$?" != "0" ]; then
    exit 1
fi

echo "enabling service..."
systemctl enable rdi-live
if [ "$?" != "0" ]; then
    exit 1
fi

echo "starting service..."
systemctl start rdi-live && sync
if [ "$?" != "0" ]; then
    exit 1
fi

exit 0
