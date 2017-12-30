package git

import (
	"fmt"
	"time"
)

type CommitOptions struct {
	All   bool
	Patch bool

	ResetAuthor bool
	Date        time.Time
	Author      Person

	Signoff           bool
	NoVerify          bool
	AllowEmpty        bool
	AllowEmptyMessage bool
	Amend             bool

	NoPostRewrite bool
	Include       bool
	Only          bool
	Quiet         bool

	// Should be passed to CommitTree, which needs support first:
	// GPGSign GPGKeyID
	// NoGpgSign bool

	// Things that are used to create the commit message and need to be
	// parsed by package cmd/, but not included here.
	//	ReuseMessage, ReeditMessage, Fixup, Squash Commitish
	// File string
	// Message string (-m)
	// Template File (COMMIT_EDITMSG)
	// Cleanup=Mode
	// Edit, NoEdit bool
	// Status, NoStatus bool
	// Verbose bool

	//

	// Things that only affect the output with --dry-run.
	// Note: Printing the status after --dry-run isn't implemented,
	// all it does is prevent the call to UpdateRef after CommitTree.
	// Most of these are a no-op.
	DryRun        bool
	Short         bool
	Branch        bool
	Porcelain     bool
	Long          bool
	NullTerminate bool
	UntrackedMode StatusUntrackedMode

	// FIXME: Add all the missing options here.
}

// Commit implements the command "git commit" in the repository pointed
// to by c.
func Commit(c *Client, opts CommitOptions, message string, files []File) (CommitID, error) {
	if !opts.AllowEmptyMessage && message == "" {
		return CommitID{}, fmt.Errorf("Aborting commit due to empty commit message.")
	}
	if opts.Patch {
		return CommitID{}, fmt.Errorf("Commit --patch not implemented")
	}
	if len(files) != 0 {
		return CommitID{}, fmt.Errorf("Commit files not implemented")
	}
	if opts.All {
		var tostage []File
		if opts.Include {
			tostage = files
		}
		if _, err := Add(c, AddOptions{Update: true, DryRun: opts.DryRun}, tostage); err != nil {
			return CommitID{}, err
		}
	}

	// Happy path: write the tree
	treeid, err := WriteTree(c, WriteTreeOptions{})
	if err != nil {
		return CommitID{}, err
	}
	// Write the commit object
	var parents []CommitID
	oldHead, err := c.GetHeadCommit()
	if err == nil || err == DetachedHead {
		parents = append(parents, oldHead)
	}
	cid, err := CommitTree(c, CommitTreeOptions{}, TreeID(treeid), parents, message)
	if err != nil {
		return CommitID{}, err
	}

	// Update the reference
	var refmsg string
	if len(message) < 50 {
		refmsg = message
	} else {
		refmsg = message[:50]
	}
	refmsg = fmt.Sprintf("commit: %s (dgit)", refmsg)

	if err := UpdateRef(c, UpdateRefOptions{OldValue: oldHead, CreateReflog: true}, "HEAD", cid, refmsg); err != nil && err != DetachedHead {
		return CommitID{}, err
	}
	return cid, nil
}
