#!/bin/bash

pbdir=$PWD/model
mkdir -p $pbdir
rm -rf $pbdir/*
tmpdir=$(mktemp -d)

# Generate protobuf message code
for proto in $(find proto -iname "*.proto"); do
    protoc \
        --go_out=$tmpdir $proto
done

# Move code to repo
mv $tmpdir/proto/* $pbdir/
rm -rf $tmpdir

exit 0
