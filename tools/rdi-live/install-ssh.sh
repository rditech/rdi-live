#!/bin/bash

# set up options
addr=""

print_usage() {
    printf 'Usage: '
    printf "$0"
    printf ' [OPTIONS] IP_ADDRESS\n'
    printf '\n'
    printf 'This tool is for installing live display software onto a DAQ computer that is connected over the network.\n'
    printf '\n'
    printf 'OPTIONS:\n'
}

while getopts 'h' flag; do
    case "${flag}" in
        *) print_usage
           exit 1 ;;
        esac
done

addr=${@:$((OPTIND)):1}
if [ -z "$addr" ]; then
    print_usage
    exit 1
fi

echo "stopping service..."
ssh root@$addr 'systemctl stop rdi-live'

echo "building the executable..."
GOARCH=amd64 GOOS=linux go build
if [ "$?" != "0" ]; then
    exit 1
fi

bin=/usr/local/bin
echo "creating $bin..."
ssh root@$addr "mkdir -p $bin"
if [ "$?" != "0" ]; then
    exit 1
fi

echo "copying executable..."
scp rdi-live root@$addr:$bin/
if [ "$?" != "0" ]; then
    exit 1
fi

echo "installing rdi-live service..."
cat rdi-live.service | ssh root@$addr 'cat > /etc/systemd/system/rdi-live.service'
if [ "$?" != "0" ]; then
    exit 1
fi
ssh root@$addr 'systemctl daemon-reload'
if [ "$?" != "0" ]; then
    exit 1
fi

echo "enabling service..."
ssh root@$addr 'systemctl enable rdi-live'
if [ "$?" != "0" ]; then
    exit 1
fi

echo "starting service..."
ssh root@$addr 'systemctl start rdi-live'
if [ "$?" != "0" ]; then
    exit 1
fi

exit 0
