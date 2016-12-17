package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	//"io/ioutil"
	"flag"
	"os"
)

func Merge(c *Client, repo *libgit.Repository, args []string) error {
	os.Args = append([]string{"git merge"}, args...)
	ffonly := flag.Bool("ff-only", false, "Only allow fast-forward merges")
	flag.Parse()
	args = flag.Args()

	if len(args) < 1 {
		flag.Usage()
		return fmt.Errorf("Invalid usage.")
	}

	commits, err := RevParse(c, append([]string{"HEAD"}, args...))
	if err != nil {
		return err
	}
	if len(commits) < 2 {
		return fmt.Errorf("Not enough arguments to merge.")
	}
	head := commits[0]

	// Check if all commits are already in head
	needsMerge := false
	for _, commit := range commits[1:] {
		if !commit.IsAncestor(c, head) {
			needsMerge = true
			break
		}
	}
	if !needsMerge {
		return fmt.Errorf("Already up-to-date.")

	}

	if len(commits) != 2 {
		return fmt.Errorf("Only fast-forward commits are currently implemented.")
	}

	// RevParse returns ParsedRevisions, which are Commitish, but slices
	// can't be passed in terms of interfaces without converting them
	// first.
	var asCommitish []Commitish
	for _, c := range commits {
		asCommitish = append(asCommitish, c)
	}

	// Find the mergebase
	base, err := MergeBaseOctopus(c, asCommitish)
	if err != nil {
		return err
	}

	// Convert the ParsedRevisions into CommitIDs
	baseCommit, err := base.CommitID(c)
	if err != nil {
		return err
	}
	headCommit, err := commits[0].CommitID(c)
	if err != nil {
		return err
	}

	// Check if it's a fast-forward commit. If the merge base is HEAD,
	// it's a fast-forward
	if baseCommit == headCommit {
		// A fast-forward commit is easy, just update the ref.
		UpdateRef(c, []string{"HEAD", commits[1].Id.String()})

		// And update the index
		ReadTree(c, []string{"HEAD"})

		// then update the the working directory.
		Checkout(c, []string{"HEAD"})
		return nil
	}
	if *ffonly {
		return fmt.Errorf("Not a fast-forward commit.")
	}

	panic("Only fast-forward commits are implemented")
}
