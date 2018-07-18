#!/bin/sh

set -e

# This is the tag of the release that dgit originated
TAG=v2.8.0

d=`dirname $0`
cd "$d"

test=$1

go build

git clone https://github.com/git/git.git official-git || echo "Using existing official git"
cd official-git
git checkout v2.8.0
make
rm git
cp ../dgit git
rm git-init
cp ../git-init .
chmod a+x git-init
cd t

make -i $test
