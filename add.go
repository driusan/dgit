package main

import (
	//"fmt"
	"os"
)

func Add(c *Client, args []string) error {
	gindex, _ := c.GitDir.ReadIndex()
	for _, arg := range args {
		f := File(arg)
		if !f.Exists() {
			gindex.RemoveFile(arg)
			continue
		}
		if file, err := os.Open(arg); err == nil {
			defer file.Close()
			gindex.AddFile(c, file)
		}
	}
	f, err := c.GitDir.Create(File("index"))
	if err != nil {
		return err
	}
	defer f.Close()
	return gindex.WriteIndex(f)
}
