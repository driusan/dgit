package main

import (
	libgit "github.com/driusan/git"
)

func WriteTree(c *Client, repo *libgit.Repository) string {
	idx, _ := c.GitDir.ReadIndex()
	sha1 := idx.WriteTree(repo)
	return sha1
}
