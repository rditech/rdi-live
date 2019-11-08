#!/bin/bash

# set up options
addr=""

print_usage() {
    printf 'Usage: '
    printf "$0"
    printf ' [OPTIONS] IP_ADDRESS HPS_UID OUTPUT_URL\n'
    printf '\n'
    printf 'This tool is for installing DAQ software onto an HPS that is connected over the network.\n'
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

uid=${@:$((OPTIND+1)):1}
if [ -z "$uid" ]; then
    print_usage
    exit 1
fi

url=${@:$((OPTIND+2)):1}
if [ -z "$url" ]; then
    print_usage
    exit 1
fi

echo "stopping service..."
ssh root@$addr 'systemctl stop rdi-cm-daq'

echo "building the executable for ARM..."
GOARCH=arm go build
if [ "$?" != "0" ]; then
    exit 1
fi

echo "creating $bin..."
bin=/usr/local/bin
ssh root@$addr "mkdir -p $bin"
if [ "$?" != "0" ]; then
    exit 1
fi

echo "copying executable..."
scp rdi-cm-daq root@$addr:$bin/
if [ "$?" != "0" ]; then
    exit 1
fi

echo "installing rdi-cm-daq service..."
sed "s/\<HPS_UID=/HPS_UID=$uid/" rdi-cm-daq.service | sed "s#\<OUTPUT_URL=#OUTPUT_URL=$url#" | ssh root@$addr 'cat > /etc/systemd/system/rdi-cm-daq.service'
if [ "$?" != "0" ]; then
    exit 1
fi
ssh root@$addr 'systemctl daemon-reload'
if [ "$?" != "0" ]; then
    exit 1
fi

echo "enabling service..."
ssh root@$addr 'systemctl enable rdi-cm-daq'
if [ "$?" != "0" ]; then
    exit 1
fi

echo "starting service..."
ssh root@$addr 'systemctl start rdi-cm-daq && sync'
if [ "$?" != "0" ]; then
    exit 1
fi

exit 0
