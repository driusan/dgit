package cmd

import (
	//"fmt"
	"github.com/driusan/go-git/git"
	"os"
)

func Add(c *git.Client, args []string) error {
	gindex, _ := c.GitDir.ReadIndex()
	for _, arg := range args {
		f := git.File(arg)
		if !f.Exists() {
			gindex.RemoveFile(arg)
			continue
		}
		if file, err := os.Open(arg); err == nil {
			defer file.Close()
			gindex.AddFile(c, file)
		}
	}
	f, err := c.GitDir.Create(git.File("index"))
	if err != nil {
		return err
	}
	defer f.Close()
	return gindex.WriteIndex(f)
}
