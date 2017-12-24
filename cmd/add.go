package cmd

import (
	"fmt"
	"github.com/driusan/dgit/git"
	"os"
)

func Add(c *git.Client, args []string) error {
	gindex, _ := c.GitDir.ReadIndex()
	for _, arg := range args {
		f := git.File(arg)
		ipath, err := f.IndexPath(c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			continue
		}

		if !f.Exists() {
			gindex.RemoveFile(ipath)
			continue
		}
		if file, err := os.Open(arg); err == nil {
			defer file.Close()
			gindex.AddFile(c, file, true)
		}
	}
	f, err := c.GitDir.Create(git.File("index"))
	if err != nil {
		return err
	}
	defer f.Close()
	return gindex.WriteIndex(f)
}
