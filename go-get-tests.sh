#!/bin/bash
set -e

echo "Running go get tests"

# Ensure that git is called, not the proxy
export GOPROXY="direct"
export GOSUMDB="off"

# Keep existing state
export ORIG_PATH=$PATH
export ORIG_GIT=$(which git)

echo "Adding dgit to the path"
go build
mkdir -p bin
cp dgit bin/git
export PATH=$(pwd)/bin:$PATH

export DGIT_TRACE=/tmp/go-get-dgit-log.$$.txt

# Make a new $GOPATH and use it so that newer versions of
# git with different default module modes don't get confused.
export GOPATH=/tmp/gopath.tst
export TEST_PKG=github.com/blang/semver
export TEST_GIT_DIR=${GOPATH}/src/${TEST_PKG}
mkdir -p $GOPATH
cd $GOPATH

echo "Go get a package inside $GOPATH"
go get -x ${TEST_PKG} || (echo "Go get ${TEST_PKG} failed"; exit 1)
test -d ${GOPATH}/src/${TEST_PKG} || (echo "ERROR: Go get didn't work"; exit 1)

test -f $DGIT_TRACE || (echo "ERROR: Dgit wasn't called for the go get test"; exit 1)
rm -f $DGIT_TRACE

echo "Reset the package back one commit from master"
$ORIG_GIT -C ${TEST_GIT_DIR} reset HEAD^1 > /dev/null
$ORIG_GIT -C ${TEST_GIT_DIR} checkout . > /dev/null
commitid=$($ORIG_GIT -C ${TEST_GIT_DIR} log --pretty=format:"%h" HEAD^..HEAD)

echo "Run go get -u on the package"
go get -x -u ${TEST_PKG} || (echo "Go get -u failed"; exit 1)
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
