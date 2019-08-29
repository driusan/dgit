#!/bin/bash
set -e

echo "Running go get tests with Go modules enabled"

# Keep existing state
export ORIG_PATH=$PATH
export ORIG_GIT=$(which git)
export ORIG_GOPATH=$GOPATH

echo "Adding dgit to the path"
go build
mkdir -p bin
cp dgit bin/git
export PATH=$(pwd)/bin:$PATH

export DGIT_TRACE=/tmp/go-get-dgit-log.$$.txt

# Use a fresh GOPATH
mkdir /tmp/gopath.$$
export GOPATH=/tmp/gopath.$$
export GO111MODULE=on # Force Go 1.11 to use the go modules

# Force Go master (>=1.14) to use git instead of a proxy.
export GOPROXY="direct"

mkdir -p /tmp/foo.$$
cd /tmp/foo.$$
go mod init somesite.com/foo || exit 0

echo "Go get a package without semver"
go get "github.com/shurcooL/vfsgen" || (echo "Go get failed"; exit 1)
test -d $GOPATH/pkg/mod/github.com/shurcoo\!l || (echo "ERROR: Go get didn't work"; exit 1)

test -f $DGIT_TRACE || (echo "ERROR: Dgit wasn't called for the go get test"; exit 1)
rm -f $DGIT_TRACE

echo "Go get a package with semver"
go get "golang.org/x/text" || (echo "Go get failed"; exit 1)
# Only test the parent directory, because the exact package directory includes @version
test -d $GOPATH/pkg/mod/golang.org/x || (echo "ERROR: Go get didn't work"; exit 1)

test -f $DGIT_TRACE
rm -f $DGIT_TRACE

export PATH=$ORIG_PATH
unset GO111MODULE
export GOPATH=$ORIG_GOPATH
