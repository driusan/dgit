package main

import (
	libgit "github.com/driusan/git"
)

func WriteTree(repo *libgit.Repository) string {
	idx, _ := ReadIndex(repo)
	sha1 := idx.WriteTree(repo)
	return sha1
}
