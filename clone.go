package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	"os"
	"strings"
)

func Clone(c *Client, args []string) {
	var repoid string
	var dirName string
	// TODO: This argument parsing should be smarter and more
	// in line with how cgit does it.
	switch len(args) {
	case 0:
		fmt.Fprintln(os.Stderr, "Usage: go-git clone repo [directory]")
		return
	case 1:
		repoid = args[0]
	case 2:
		repoid = args[0]
		dirName = args[1]
	default:
		repoid = args[0]
	}
	repoid = strings.TrimRight(repoid, "/")
	pieces := strings.Split(repoid, "/")

	if dirName == "" {
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
	}

	c = Init(c, []string{dirName})
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		panic(err)
	}
	Config(c, []string{"--set", "remote.origin.url", repoid})
	Config(c, []string{"--set", "branch.master.remote", "origin"})

	// This should be smarter and try and get the HEAD branch from Fetch.
	// The HEAD refspec isn't necessarily named refs/heads/master.
	Config(c, []string{"--set", "branch.master.merge", "refs/heads/master"})

	Fetch(c, repo, []string{"origin"})
	SymbolicRef(c, []string{"HEAD", "refs/heads/master"})
	Reset(c, []string{"--hard"})
}
