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

make t0000-basic.sh \
        t0004-unwritable.sh \
        t0007-git-var.sh \
        t0008-ignores.sh \
        t0010-racy-git.sh \
        t0062-revision-walking.sh \
        t0070-fundamental.sh \
        t0081-line-buffer.sh \
        t1000-read-tree-m-3way.sh \
        t1001-read-tree-m-2way.sh \
        t1002-read-tree-m-u-2way.sh \
        t1003-read-tree-prefix.sh \
        t1004-read-tree-m-u-wf.sh \
        t1005-read-tree-reset.sh \
        t1008-read-tree-overlay.sh \
        t1009-read-tree-new-index.sh \
        t1011-read-tree-sparse-checkout.sh \
        t1012-read-tree-df.sh \
        t1014-read-tree-confusing.sh \
        t1100-commit-tree-options.sh \
        t1304-default-acl.sh \
        t1308-config-set.sh \
        t1403-show-ref.sh \
        t1500-rev-parse.sh \
        t2000-checkout-cache-clash.sh \
        t2001-checkout-cache-clash.sh \
        t2002-checkout-cache-u.sh \
        t2006-checkout-index-basic.sh \
        t2009-checkout-statinfo.sh \
        t2014-switch.sh \
        t2018-checkout-branch.sh \
        t2100-update-cache-badpath.sh \
        t3000-ls-files-others.sh \
        t3002-ls-files-dashpath.sh \
        t3006-ls-files-long.sh \
        t3020-ls-files-error-unmatch.sh \
        t3800-mktag.sh \
        t4113-apply-ending.sh \
        t4123-apply-shrink.sh \
        t5510-fetch.sh \
        t7062-wtstatus-ignorecase.sh \
        t7511-status-index.sh

