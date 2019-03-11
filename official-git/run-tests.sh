#!/bin/bash

# Runs the offical git command line scripts from a specific version.
# Command line arguments to this script are the ones given to make
#  in the git/t directory where the test cases are defined.
# E.g. ./run-official-tests.sh -i t0000-basic.sh
# Runs the tests defined in the shell script and ignores failures (-i)

set -e

# This is the tag of the release that dgit originated
TAG=v2.10.0

d=`dirname $0`

cd $d
d=`pwd`

cd $d/..; go build

cd $d
git clone https://github.com/git/git.git git || echo "Using existing official git"
cd git
git checkout "$TAG"
patch -p1 -N < "$d/fix-checkout-branch.patch" || echo "Fix checkout branch tests patch already applied"
git apply ../force-official-git-pack-objects.patch || echo "Force official git pack objects patch already applied"
make
test -f git.official || cp git git.official
rm git
cp ../../dgit git
rm git-init
cp ../git-init .
chmod a+x git-init
rm git-ls-remote

sed s/init/ls-remote/g git-init > git-ls-remote
chmod a+x git-ls-remote

rm git-remote
sed s/init/remote/g git-remote > git-remote
chmod a+x git-remote
cd t

# t0008 tests that are skipped require ! to negate a pattern. (GitHub issue #72)
# t1004.16 needs "git merge-resolve" (which isn't documented anywhere I can find)
# t1004.17 needs "git merge-recursive" (which also isn't documented)
# t1014.26 needs "git config --unset-all"
# t3000.7 requires git pack-refs

GIT_SKIP_TESTS="t0008.321 t0008.323 t0008.37[0-9] t0008.38[0-7] t0008.39[1-2]"
GIT_SKIP_TESTS="$GIT_SKIP_TESTS t1004.1[6-7]"
GIT_SKIP_TESTS="$GIT_SKIP_TESTS t1014.26"
GIT_SKIP_TESTS="$GIT_SKIP_TESTS t1308.2[6-7]" # No support for line ending handling or GIT_CEILING_DIRECTORIES
GIT_SKIP_TESTS="$GIT_SKIP_TESTS t1503.[5-7] 1503.10" # No support for @{suffix} (looking up based on reflog) in rev-parse
GIT_SKIP_TESTS="$GIT_SKIP_TESTS t2018.[6-7] t2018.[9] t2018.1[5-8]"
GIT_SKIP_TESTS="$GIT_SKIP_TESTS t3000.7"
GIT_SKIP_TESTS="$GIT_SKIP_TESTS t5510.[4-9] t5510.[1-7][0-9]" # Just the basic fetch tests are working for now
export GIT_SKIP_TESTS

echo t0000-basic
./t0000-basic.sh
echo t0004-unwritable
./t0004-unwritable.sh
echo t0007-git-var
./t0007-git-var.sh
echo t0008-ignores
./t0008-ignores.sh
echo t0010-racy-git
./t0010-racy-git.sh
echo t0062-revision-walking
./t0062-revision-walking.sh
echo t0070-fundamental
./t0070-fundamental.sh
echo t0081-line-buffer
./t0081-line-buffer.sh
echo t1000-read-tree-m-3way
./t1000-read-tree-m-3way.sh
echo t1001-read-tree-m-2way
./t1001-read-tree-m-2way.sh
echo t1002-read-tree-m-u-2way
./t1002-read-tree-m-u-2way.sh
echo t1003-read-tree-prefix
./t1003-read-tree-prefix.sh
echo t1004-read-tree-m-u-wf
./t1004-read-tree-m-u-wf.sh
echo t1005-read-tree-reset
./t1005-read-tree-reset.sh
echo t1008-read-tree-overlay
./t1008-read-tree-overlay.sh
echo t1009-read-tree-new-index
./t1009-read-tree-new-index.sh
echo t1011-read-tree-sparse-checkout
./t1011-read-tree-sparse-checkout.sh
echo t1012-read-tree-df
./t1012-read-tree-df.sh
echo t1014-read-tree-confusing
./t1014-read-tree-confusing.sh
echo t1100-commit-tree-options
./t1100-commit-tree-options.sh
echo t1304-default-acl
./t1304-default-acl.sh
echo t1308-config-set
./t1308-config-set.sh
echo t1403-show-ref
./t1403-show-ref.sh
echo t1500-rev-parse
./t1500-rev-parse.sh
echo t2000-checkout-cache-clash
./t2000-checkout-cache-clash.sh
echo t2001-checkout-cache-clash
./t2001-checkout-cache-clash.sh
echo t2002-checkout-cache-u
./t2002-checkout-cache-u.sh
echo t2006-checkout-index-basic
./t2006-checkout-index-basic.sh
echo t2009-checkout-statinfo
./t2009-checkout-statinfo.sh
echo t2014-switch
./t2014-switch.sh
echo t2018-checkout-branch
./t2018-checkout-branch.sh
echo t2100-update-cache-badpath
./t2100-update-cache-badpath.sh
echo t3000-ls-files-others
./t3000-ls-files-others.sh
echo t3002-ls-files-dashpath
./t3002-ls-files-dashpath.sh
echo t3006-ls-files-long
./t3006-ls-files-long.sh
echo t3020-ls-files-error-unmatch
./t3020-ls-files-error-unmatch.sh
echo t3800-mktag
./t3800-mktag.sh
echo t4113-apply-ending
./t4113-apply-ending.sh
echo t4123-apply-shrink
./t4123-apply-shrink.sh
echo t5510-fetch.sh
export DGIT_TRACE=/tmp/dgit-trace.txt
rm -f $DGIT_TRACE
./t5510-fetch.sh -d -v -i
cat $DGIT_TRACE
echo t7062-wtstatus-ignorecase
./t7062-wtstatus-ignorecase.sh
echo t7511-status-index
./t7511-status-index.sh

