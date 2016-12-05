package main

import (
	//"fmt"
	libgit "github.com/driusan/git"
	"os"
)

func Add(c *Client, repo *libgit.Repository, args []string) {
	gindex, _ := ReadIndex(c, repo)
	for _, arg := range args {
		if _, err := os.Stat(arg); os.IsNotExist(err) {
			gindex.RemoveFile(repo, arg)
			continue
		}
		if file, err := os.Open(arg); err == nil {
			defer file.Close()
			gindex.AddFile(repo, file)
		}
	}
	writeIndex(repo, gindex, "index")

}
