package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	"os"
	"strings"
)

func Clone(repo *libgit.Repository, args []string) {
	var repoid string
	// TODO: This argument parsing should be smarter and more
	// in line with how cgit does it.
	switch len(args) {
	case 0:
		fmt.Fprintln(os.Stderr, "Usage: go-git clone repo [directory]")
		return
	case 1:
		repoid = args[0]
	default:
		repoid = args[0]
	}
	repoid = strings.TrimRight(repoid, "/")
	pieces := strings.Split(repoid, "/")

	var dirName string
	if len(pieces) > 0 {
		dirName = pieces[len(pieces)-1]
	}
	dirName = strings.TrimSuffix(dirName, ".git")

	if _, err := os.Stat(dirName); err == nil {
		fmt.Fprintf(os.Stderr, "Directory %s already exists, can not clone.\n", dirName)
		return
	}
	if dirName == "" {
		panic("No directory left to clone into.")
	}

	if repo == nil {
		repo = &libgit.Repository{}
	}

	Init(repo, []string{dirName})

	Config(repo, []string{"--set", "remote.origin.url", repoid})
	Config(repo, []string{"--set", "branch.master.remote", "origin"})

	Fetch(repo, []string{"origin"})
	Reset(repo, []string{"--hard"})
}
