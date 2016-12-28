package git

import (
	"fmt"
)

// CheckoutOptions represents the options that may be passed to
// "git checkout"
type CheckoutOptions struct {
	// Not implemented
	Quiet bool
	// Not implemented
	Progress bool
	// Not implemented
	Force bool

	// Check out the named stage for unnamed paths.
	// Stage2 is equivalent to --ours, Stage3 to --theirs
	// Not implemented
	Stage Stage

	// Not implemented
	Branch string // -b
	// Not implemented
	ForceBranch bool // use branch as -B
	// Not implemented
	OrphanBranch bool // use branch as --orphan

	// Not implemented
	Track string
	// Not implemented
	CreateReflog bool // -l

	// Not implemented
	Detach bool

	// Not implemented
	IgnoreSkipWorktreeBits bool
	// Not implemented
	Merge bool

	// Not implemented.
	ConflictStyle string

	// Not implemented
	Patch bool

	// Not implemented
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
	if thing == "" {
		thing = "HEAD"
	}

	if len(files) == 0 {
		cmt, err := RevParseCommit(c, &RevParseOptions{}, thing)
		if err != nil {
			return err
		}
		return CheckoutCommit(c, opts, cmt)
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
	if opts.Branch != "" {
		// commit is the startpoint in the last variation, otherwise
		// Checkout() already set it to the commit of "HEAD"
		refspec := RefSpec("refs/heads/" + opts.Branch)
		refspecfile := refspec.File(c)
		if refspecfile.Exists() && !opts.ForceBranch {
			return fmt.Errorf("Branch %s already exists.", opts.Branch)
		}
		return fmt.Errorf("CheckoutCommit not yet implemented")
	}
	cmt, err := commit.CommitID(c)
	if err != nil {
		return err
	}
	return CheckoutFiles(c, opts, cmt, nil)
}

// Implements "git checkout" subcommand of git for variations:
//     git checkout [-f|--ours|--theirs|-m|--conflict=<style>] [<tree-ish>] [--] <paths>...
//     git checkout [-p|--patch] [<tree-ish>] [--] [<paths>...]
func CheckoutFiles(c *Client, opts CheckoutOptions, tree Treeish, files []string) error {
	// If files were specified, we don't want ReadTree to update the workdir,
	// because we only want to (force) update the specified files.
	//
	// If they weren't, we want to checkout a treeish, so let ReadTree update
	// the workdir so that we don't lose any changes.
	updateAll := (len(files) == 0)
	i, err := ReadTree(c, ReadTreeOptions{Update: updateAll, Merge: updateAll}, tree)
	if err != nil {
		return err
	}
	// ReadTree wrote the index to disk, but since we already have a copy in
	// memory we use the Uncommited variation.
	return CheckoutIndexUncommited(c, i, CheckoutIndexOptions{Force: true}, files)
}
