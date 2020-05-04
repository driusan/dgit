#!/bin/bash
set -e

echo "Running go get tests"

export GOPROXY="direct"

# Keep existing state
export ORIG_PATH=$PATH
export ORIG_GIT=$(which git)

export TEST_PKG=github.com/golang/protobuf/proto
export TEST_GIT_DIR=../../golang/protobuf

echo "Adding dgit to the path"
go build
mkdir -p bin
cp dgit bin/git
export PATH=$(pwd)/bin:$PATH

export DGIT_TRACE=/tmp/go-get-dgit-log.$$.txt

echo "Go get a package"
go get -x ${TEST_PKG} || (echo "Go get failed"; exit 1)
test -d ${TEST_GIT_DIR} || (echo "ERROR: Go get didn't work"; exit 1)

test -f $DGIT_TRACE || (echo "ERROR: Dgit wasn't called for the go get test"; exit 1)
rm -f $DGIT_TRACE

echo "Reset the package back one commit from master"
$ORIG_GIT -C ${TEST_GIT_DIR} reset HEAD^1 > /dev/null
$ORIG_GIT -C ${TEST_GIT_DIR} checkout . > /dev/null
commitid=$($ORIG_GIT -C ${TEST_GIT_DIR} log --pretty=format:"%h" HEAD^..HEAD)

echo "Run go get -u on the package"
go get -u github.com/golang/protobuf/proto || (echo "Go get -u failed"; exit 1)
test -f $DGIT_TRACE || (echo "ERROR: Dgit wasn't called for the go get -u test"; exit 1)

echo "Verify that the branch is now up to date with master"
commitid2=$($ORIG_GIT -C ${TEST_GIT_DIR} log --pretty=format:"%h" HEAD^..HEAD)

echo COMMITS: "$commitid" "$commitid2"
if [ "$commitid" == "$commitid2" ]
then
        echo "ERROR: Pull did not pull in the latest changes"
        exit 1
fi

export PATH=$ORIG_PATH
