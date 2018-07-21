package git

import (
	"fmt"
	"io/ioutil"
)

type RevertOptions struct {
	Edit bool

	MergeParent int

	NoCommit bool
	SignOff  bool

	MergeStrategy       string
	MergeStrategyOption string

	Continue, Quit, Abort bool
}

// Reverts the given commits from the HEAD.  Revert doesn't actually create the
// new commit, it returns the new tree and leaves it up to the caller (who has
// a better chance of knowing the appropriate commit message..)
func Revert(c *Client, opts RevertOptions, commits []Commitish) error {
	switch {
	case len(commits) == 0:
		return fmt.Errorf("Must provide commit to revert")
	case len(commits) > 1:
		return fmt.Errorf("Only 1 revert at a time currently supported. Sequencer not implemented")
	}

	// Ensure that the tree is clean before doing anything.
	worktree, err := DiffFiles(c, DiffFilesOptions{}, nil)
	if err != nil {
		return err
	}
	if len(worktree) > 0 {
		return fmt.Errorf("Untracked changes to files would be modified. Aborting")
	}
	head, err := c.GetHeadCommit()
	if err != nil {
		return err
	}

	if err := UpdateRef(c, UpdateRefOptions{NoDeref: true}, "REVERT_HEAD", head, ""); err != nil {
		return err
	}

	cmt, err := commits[0].CommitID(c)
	if err != nil {
		return err
	}
	parents, err := cmt.Parents(c)
	if err != nil {
		return err
	}
	var diff []HashDiff
	if len(parents) == 0 {
		// FIXME: This shouldn't be the case, but needs to be handled
		// as a special case.
		return fmt.Errorf("Can not revert initial commit.")
	} else if len(parents) == 1 {
		diffs, err := DiffTree(c, &DiffTreeOptions{}, parents[0], cmt, nil)
		if err != nil {
			return err
		}
		diff = diffs
	} else if opts.MergeParent <= 0 {
		return fmt.Errorf("Must specify merge parent to revert a merge commit.")
	} else {
		// the merge parent is 1 indexed.
		diffs, err := DiffTree(c, &DiffTreeOptions{}, parents[opts.MergeParent-1], cmt, nil)
		if err != nil {
			return err
		}
		diff = diffs
	}

	patch, err := ioutil.TempFile("", "gitrevertpatch")
	if err != nil {
		return err
	}
	if err := GeneratePatch(c, DiffCommonOptions{Patch: true}, diff, patch); err != nil {
		return err
	}
	if err := Apply(c, ApplyOptions{ThreeWay: true, Reverse: true, Index: true}, []File{File(patch.Name())}); err != nil {
		return err
	}
	return nil
}
