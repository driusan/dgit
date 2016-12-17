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

	// If --ff-only is specified, verify it is in fact a fast-forward.
	if *ffonly {
		for _, commit := range commits[1:] {
			if !head.IsAncestor(c, commit) {
				return fmt.Errorf("Not a fast-forward commit.")
			}
		}
		// A fast-forward commit is easy, just update the ref.
		UpdateRef(c, []string{"HEAD", commits[1].Id.String()})

		// And update the index
		ReadTree(c, []string{"HEAD"})
		return nil
	}

	panic("Only fast-forward commits are implemented")
	/*
		var commits
		for _, commit := range commits {
		}
		mergebase, err := MergeBase(c, repo, commits)
		if err != nil {
			return err
		}
		fmt.Printf("Mergebase: %v\n", mergebase)
		return nil
	*/
	/*
		if len(args) != 1 {
			fmt.Fprintf(os.Stderr, "Usage: go-git merge branchname\n")
			return
		}
		commit, err := repo.GetCommitOfBranch(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid branch: %s\n", args[0])
			return
		}
		head, err := c.GetHeadID()
		if err != nil {
			panic(err)
		}

		// The current branch is an ancestor of HEAD. This
		// is a fast-forward.
		if headCom, err := repo.GetCommit(head); err == nil {
			if isAncestor(headCom, fmt.Sprintf("%s", commit.Id)) {
				fmt.Fprintf(os.Stderr, "Already up-to-date.\n")
				return
			}
		}

		// Head is an ancestor of the current branch.
		if isAncestor(commit, head) {
			hb := c.GetHeadBranch()
			newId := fmt.Sprintf("%s", commit.Id)

			ioutil.WriteFile(".git/refs/heads/"+hb, []byte(newId), 0644)
			resetIndexFromCommit(c, newId)
			fmt.Printf("Hooray! A Fast forward on %s! New should should be %s\n", hb, commit.Id)
			return
		}

		panic("Only fast forward commits are currently supported.")*/
}
