package main

import (
	libgit "github.com/driusan/git"
)

func WriteTree(repo *libgit.Repository) {
	idx, _ := ReadIndex(repo)
	idx.WriteTree(repo)
}
