package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	"os"
)

func Branch(repo *libgit.Repository, args []string) {
	switch len(args) {
	case 0:
		branches, err := repo.GetBranches()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not get list of branches.")
			return
		}
		head := getHeadBranch(repo)
		for _, b := range branches {
			if head == b {
				fmt.Print("* ")
			} else {
				fmt.Print("  ")
			}
			fmt.Println(b)
		}
	case 1:
		if head, err := getHeadId(repo); err == nil {
			if cerr := libgit.CreateBranch(repo.Path, args[0], head); cerr != nil {
				fmt.Fprintf(os.Stderr, "Could not create branch: %s\n", cerr.Error())
			}
		} else {
			fmt.Fprintf(os.Stderr, "Could not create branch: %s\n", err.Error())
		}
	default:
		fmt.Fprintln(os.Stderr, "Usage: go-git branch [branchname]")
	}

}
