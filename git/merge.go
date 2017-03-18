package git

import (
	"fmt"
	"io"
	"os"
)

type MergeStrategy string

func commitishName(c Commitish) string {
	if b, ok := c.(Branch); ok {
		return b.BranchName()
	}
	return ""
}

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

		return UpdateRef(c, UpdateRefOptions{OldValue: head}, "HEAD", dstc, refmsg)
	}

	if opts.FastForwardOnly {
		return fmt.Errorf("Not a fast-forward commit.")
	}

	if len(others) != 1 {
		return fmt.Errorf("Can only merge one branch at a time (for now.)")
	}

	// Perform a three-way merge with mergebase, head, and tree.
	tree, err := others[0].CommitID(c)
	if err != nil {
		return err
	}

	idx, err := ReadTreeMerge(c,
		ReadTreeOptions{
			Merge:  true,
			Update: true,
		},
		base,
		head,
		tree,
	)
	if err != nil {
		return err
	}

	// run MergeFile on any unmerged entries.
	unmerged := idx.GetUnmerged()
	if len(unmerged) != 0 {
		// Flag conflicts in the tree if necessary.
		conflictLabel := commitishName(others[0])
		if conflictLabel == "" {
			conflictLabel = tree.String()
		}
		var errStr string
		for path, file := range unmerged {
			fp, err := path.FilePath(c)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Auto-merging %v\n", fp)

			// Check out the stages to temporary files for MergeFile
			stage1tmp, _ := checkoutTemp(c, file.Stage1, CheckoutIndexOptions{})
			defer os.Remove(stage1tmp)
			stage2tmp, _ := checkoutTemp(c, file.Stage2, CheckoutIndexOptions{})
			defer os.Remove(stage2tmp)
			stage3tmp, _ := checkoutTemp(c, file.Stage3, CheckoutIndexOptions{})
			defer os.Remove(stage3tmp)

			// run git merge-file with the appropriate parameters.
			r, err := MergeFile(c,
				MergeFileOptions{
					Current: MergeFileFile{
						Filename: File(stage2tmp),
						Label:    "HEAD",
					},
					Base: MergeFileFile{
						Filename: File(stage1tmp),
						Label:    "merged common ancestors",
					},
					Other: MergeFileFile{
						Filename: File(stage3tmp),
						Label:    conflictLabel,
					},
				},
			)
			if err != nil {
				errStr += "CONFLICT (content): Merge conflict in " + fp.String() + "\n"
			}

			// Write the output with conflict markers into the file.
			f2, err := os.Create(fp.String())
			if err != nil {
				return err
			}

			io.Copy(f2, r)
			f2.Close()
		}

		// Only error out if there was at least 1 conflict, otherwise it was
		// a success.
		if errStr != "" {
			return fmt.Errorf("%vAutomatic merge failed; fix conflicts and then commit the result.", errStr)
		}
	}

	// TODO: Finally, create the new commit.
	return nil
}
