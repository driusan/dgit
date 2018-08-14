#!/bin/bash
set -e

echo "Running go get tests"

# Keep existing state
export ORIG_PATH=$PATH
export ORIG_GIT=$(which git)

export TEST_PKG=github.com/golang/protobuf/proto
export TEST_GIT_DIR=../../golang/protobuf

echo "Adding dgit to the path"
go build
mkdir bin
cp dgit bin/git
export PATH=$(pwd)/bin:$PATH

export DGIT_TRACE=/tmp/go-get-dgit-log.$$.txt

echo "Go get a package"
go get ${TEST_PKG} || echo "Known problem with go get, it invokes the submodule command"
test -d ${TEST_GIT_DIR} || (echo "ERROR: Go get didn't work"; exit 1)

test -f $DGIT_TRACE || (echo "ERROR: Dgit wasn't called for the go get test"; exit 1)
unset DGIT_TRACE

echo "Reset the package back one commit from master"
$ORIG_GIT -C ${TEST_GIT_DIR} reset HEAD^1
$ORIG_GIT -C ${TEST_GIT_DIR} checkout .

# UNCOMMENT THE FOLLOWING ONCE PULL IS IMPLEMENTED
git -C ${TEST_GIT_DIR} fetch origin
git -C ${TEST_GIT_DIR} merge origin/master
#echo "Run go get -u on the package"
#go get -u github.com/golang/protobuf/proto

#echo "Verify that the branch is now up to date with master"
#$ORIG_GIT -C ${TEST_GIT_DIR} status | grep "Your branch is up to date with 'origin/master'." || (echo "ERROR: Update didn't work"; exit 1)

export PATH=$ORIG_PATH
