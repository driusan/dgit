package main

import (
	"fmt"
	"os"

	libgit "github.com/driusan/git"
)

func (g *GitIndex) ResetIndex(c *Client, tree Treeish) error {
	// TODO: Fix this or move it to client_hacks.go
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return err
	}

	treeId, err := tree.TreeID(c)
	if err != nil {
		return err
	}
	sha, err := libgit.NewId(treeId[:])
	if err != nil {
		return err
	}

	t := libgit.NewTree(repo, sha)
	if tree == nil {
		panic("Error retriving tree for commit")
	}

	newEntries, err := expandGitTreeIntoIndexes(repo, t, "", true, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resetting index: %s\n", err)
		return err
	}
	fmt.Printf("%s", newEntries)
	g.NumberIndexEntries = uint32(len(newEntries))
	g.Objects = newEntries
	return nil
}
