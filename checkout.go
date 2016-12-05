package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

func Checkout(c *Client, args []string) {
	switch len(args) {
	case 0:
		fmt.Fprintf(os.Stderr, "Missing argument for checkout")
		return
	}

	idx, _ := c.GitDir.ReadIndex()
	for _, file := range args {
		fmt.Printf("Doing something with %s\n", file)
		f := File(file)
		if !f.Exists() {
			continue
		}
		for _, idxFile := range idx.Objects {
			if idxFile.PathName == file {
				obj, err := c.GetObject(idxFile.Sha1)
				if err != nil {
					panic("Couldn't load object referenced in index.")
				}

				fmode := os.FileMode(idxFile.Mode)
				err = ioutil.WriteFile(file, obj.GetContent(), fmode)
				if err != nil {
					panic("Couldn't write file" + file)
				}
				os.Chmod(file, os.FileMode(idxFile.Mode))
			}
		}

	}
}
