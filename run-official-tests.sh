#!/bin/sh

# Runs the offical git command line scripts from a specific version.
# Command line arguments to this script are the ones given to make
#  in the git/t directory where the test cases are defined.
# E.g. ./run-official-tests.sh -i t0000-basic.sh
# Runs the tests defined in the shell script and ignores failures (-i)

set -e

# This is the tag of the release that dgit originated
TAG=v2.8.0

d=`dirname $0`
cd "$d"

test=$1

go build

git clone https://github.com/git/git.git official-git || echo "Using existing official git"
cd official-git
git checkout "$TAG"
patch -p0 < ../official-git.patch || echo "Patch already applied or conflicts with local changes"
make
rm git
cp ../dgit git
rm git-init
cp ../git-init .
chmod a+x git-init
cd t

make "$@"
