package main

import (
	//"fmt"
	"os"
)

func Add(c *Client, args []string) {
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
	writeIndex(c, gindex, "index")

}
