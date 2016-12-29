package git

import (
	"fmt"
)

type MergeStrategy string

const (
	MergeRecursive = MergeStrategy("recursive")
	MergeOctopus   = MergeStrategy("octopus")
)

// Merge options represent the options that may be passed on
// the command line to "git merge"
type MergeOptions struct {
	// Not implemented
	NoCommit bool

	// Not implemented
	NoEdit bool

	NoFastForward   bool
	FastForwardOnly bool

	// Not implemented
	Log int
	// Not implemented
	Stat bool

	// Not implemented
	Squash bool

	// Not implemented
	Strategy MergeStrategy

	// Not implemented
	VerifySignatures bool

	// Not implemented
	Quiet bool
	// Not implemented
	Verbose bool

	// Not implemented
	NoProgress bool

	// Not implemented
	Message string
}

// Aborts an in progress merge as "git merge --abort"
// (Not implemented)
func MergeAbort(c *Client, opt MergeOptions) error {
	return fmt.Errorf("MergeAbort not yet implemented")
}

// Implements the "git merge" porcelain command to merge other commits
// into HEAD and create a new commit.
func Merge(c *Client, opts MergeOptions, others []Commitish) error {
	if len(others) < 1 {
		return fmt.Errorf("Can't merge nothing.")
	}

	head, err := c.GetHeadCommit()
	if err != nil {
		return err
	}

	// Check if all commits are already in head
	needsMerge := false
	for _, commit := range others {
		cid, err := commit.CommitID(c)
		if err != nil {
			return err
		}
		if !cid.IsAncestor(c, head) {
			needsMerge = true
			break
		}
	}
	if !needsMerge {
		return fmt.Errorf("Already up-to-date.")

	}

	if len(others) != 1 {
		return fmt.Errorf("Only fast-forward commits are currently implemented.")
	}

	// RevParse returns ParsedRevisions, which are Commitish, but slices
	// can't be passed in terms of interfaces without converting them
	// first.
	commitsWithHead := []Commitish{head}
	commitsWithHead = append(commitsWithHead, others...)

	// Find the mergebase
	base, err := MergeBaseOctopus(c, commitsWithHead)
	if err != nil {
		return err
	}

	// Check if it's a fast-forward commit. If the merge base is HEAD,
	// it's a fast-forward
	if base == head && !opts.NoFastForward {
		dst := others[0]

		// Resolve to a CommitID to implement Treeish for ReadTree
		dstc, err := dst.CommitID(c)
		if err != nil {
			return err
		}
		_, err = ReadTreeFastForward(
			c,
			ReadTreeOptions{Merge: true, Update: true},
			head,
			dstc,
		)
		if err != nil {
			return err
		}
		var refmsg string
		if b, ok := others[0].(Branch); ok && b.BranchName() != "" {
			refmsg = fmt.Sprintf("merge %s into %s: Fast-forward (go-git)", b.BranchName(), c.GetHeadBranch().BranchName())
		} else {
			refmsg = fmt.Sprintf("merge into %s: Fast-forward (go-git)", c.GetHeadBranch().BranchName())
		}

		return UpdateRef(c, UpdateRefOptions{OldValue: head.String()}, "HEAD", dstc, refmsg)
	}
	if opts.FastForwardOnly {
		return fmt.Errorf("Not a fast-forward commit.")
	}
	return fmt.Errorf("Only fast-forward commits are implemented")
}
