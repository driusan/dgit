#!/bin/sh

d=`dirname $0`
cd "$d"

git clone https://github.com/git/git.git official-git
cd official-git
git checkout v2.8.0
make
rm git
cp ../dgit git
rm git-init
cp ../git-init .
chmod a+x git-init
cd t
../git help
../git-init --help
make t0000-basic.sh

