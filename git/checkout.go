package git

import (
	"fmt"
)

// CheckoutOptions represents the options that may be passed to
// "git checkout"
type CheckoutOptions struct {
	Quiet    bool
	Progress bool
	Force    bool

	// Check out the named stage.
	// Stage2 is equivalent to --ours, Stage3 to --theirs
	Stage Stage

	Branch       string // -b
	ForceBranch  bool   // use branch as -B
	OrphanBranch bool   // use branch as --orphan

	Track        bool
	CreateReflog bool // -l

	Detach bool

	IgnoreSkipWorktreeBits bool
	Merge                  bool

	// Not implemented.
	ConflictStyle string

	Patch bool

	IgnoreOtherWorktrees bool
}

// Implements the "git checkout" subcommand of git. Variations in the man-page
// are:
//
//     git checkout [-q] [-f] [-m] [<branch>]
//     git checkout [-q] [-f] [-m] --detach [<branch>]
//     git checkout [-q] [-f] [-m] [--detach] <commit>
//     git checkout [-q] [-f] [-m] [[-b|-B|--orphan] <new_branch>] [<start_point>]
//     git checkout [-f|--ours|--theirs|-m|--conflict=<style>] [<tree-ish>] [--] <paths>...
//     git checkout [-p|--patch] [<tree-ish>] [--] [<paths>...]
//
// This will just check the options and call the appropriate variation. You
// can avoid the overhead by calling the proper variation directly.
//
// "thing" is the thing that the user entered on the command line to be checked out. It
// might be a branch, a commit, or a treeish, depending on the variation above.
func Checkout(c *Client, opts CheckoutOptions, thing string, files []string) error {
	if len(files) == 0 {
		if sha1, err := c.GetBranchCommit(thing); err != nil {
			return CheckoutCommit(c, opts, sha1)
		}

		if opts.Branch != "" {
			b, err := RevParseCommit(c, &RevParseOptions{}, "HEAD")
			if err != nil {
				return err
			}
			return CheckoutCommit(c, opts, b)
		}
	}

	if thing == "" {
		thing = "HEAD"
	}

	b, err := RevParseTreeish(c, &RevParseOptions{}, thing)
	if err != nil {
		return err
	}
	return CheckoutFiles(c, opts, b, files)
}

// Implements the "git checkout" subcommand of git for variations:
//     git checkout [-q] [-f] [-m] [<branch>]
//     git checkout [-q] [-f] [-m] --detach [<branch>]
//     git checkout [-q] [-f] [-m] [--detach] <commit>
//     git checkout [-q] [-f] [-m] [[-b|-B|--orphan] <new_branch>] [<start_point>]
func CheckoutCommit(c *Client, opts CheckoutOptions, commit Commitish) error {
	return fmt.Errorf("Not yet implemented. Please use checkout-index directly instead.")
}

// Implements "git checkout" subcommand of git for variations:
//     git checkout [-f|--ours|--theirs|-m|--conflict=<style>] [<tree-ish>] [--] <paths>...
//     git checkout [-p|--patch] [<tree-ish>] [--] [<paths>...]
func CheckoutFiles(c *Client, opts CheckoutOptions, tree Treeish, files []string) error {
	i, err := ReadTree(c, ReadTreeOptions{Update: true, Merge: true}, tree)
	if err != nil {
		return err
	}
	// ReadTree wrote the index to disk, but since we already have a copy in
	// memory we use the Uncommited variation.
	return CheckoutIndexUncommited(c, i, CheckoutIndexOptions{Force: true}, files)
}
