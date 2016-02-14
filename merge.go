package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	"io/ioutil"
	"os"
)

func isAncestor(parent *libgit.Commit, child string) bool {
	childId, _ := libgit.NewIdFromString(child)
	if childId == parent.Id {
		return true
	}

	for i := 0; i < parent.ParentCount(); i += 1 {
		if p, err := parent.Parent(i); err == nil {
			if isAncestor(p, child) {
				return true
			}
		}
	}
	return false
}
func Merge(repo *libgit.Repository, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: go-git merge branchname\n")
		return
	}
	commit, err := repo.GetCommitOfBranch(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid branch: %s\n", args[0])
		return
	}
	head, err := getHeadId(repo)
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
		hb := getHeadBranch(repo)
		newId := fmt.Sprintf("%s", commit.Id)

		ioutil.WriteFile(".git/refs/heads/"+hb, []byte(newId), 0644)
		resetIndexFromCommit(repo, newId)
		fmt.Printf("Hooray! A Fast forward on %s! New should should be %s\n", hb, commit.Id)
		return
	}

	panic("Only fast forward commits are currently supported.")
}
