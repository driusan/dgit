package git

import (
	libgit "github.com/driusan/git"
)

// Replaces the index of Client with the the tree from the provided Treeish.
// if PreserveStatInfo is true, the stat information in the index won't be
// modified for existing entries.
func (g *Index) ResetIndex(c *Client, tree Treeish) error {
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
		return err
	}
	g.NumberIndexEntries = uint32(len(newEntries))
	g.Objects = newEntries
	return nil
}
