package cmd

import (
	"github.com/driusan/dgit/git"
)

// WriteTree implements the git write-tree command on the Git repository
// pointed to by c.
func WriteTree(c *git.Client) string {
	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		return err.Error()
	}
	sha1, err := idx.WriteTree(c)
	if err != nil {
		return err.Error()
	}
	return sha1.String()
}
