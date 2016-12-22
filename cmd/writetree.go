package cmd

import (
	"github.com/driusan/go-git/git"
)

// WriteTree implements the git write-tree command on the Git repository
// pointed to by c.
func WriteTree(c *git.Client) string {
	idx, _ := c.GitDir.ReadIndex()
	sha1 := idx.WriteTree(c)
	return sha1
}
