package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	"os"
)

func Add(repo *libgit.Repository, args []string) {
	gindex, _ := ReadIndex(repo)
	for _, arg := range args {
		if _, err := os.Stat(arg); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "File %s does not exist.\n")
			continue
		}
		if file, err := os.Open(arg); err == nil {
			gindex.AddFile(repo, file)
		}
	}
	writeIndex(repo, gindex, "index")

}
