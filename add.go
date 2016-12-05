package main

import (
	//"fmt"
	libgit "github.com/driusan/git"
	"os"
)

func Add(c *Client, repo *libgit.Repository, args []string) {
	gindex, _ := c.GitDir.ReadIndex()
	for _, arg := range args {
		if _, err := os.Stat(arg); os.IsNotExist(err) {
			gindex.RemoveFile(arg)
			continue
		}
		if file, err := os.Open(arg); err == nil {
			defer file.Close()
			gindex.AddFile(repo, file)
		}
	}
	writeIndex(c, gindex, "index")

}
