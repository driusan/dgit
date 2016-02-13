package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	"io/ioutil"
	"os"
)

func Checkout(repo *libgit.Repository, args []string) {
	switch len(args) {
	case 0:
		fmt.Fprintf(os.Stderr, "Missing argument for checkout")
		return
	}

	idx, _ := ReadIndex(repo)
	for _, file := range args {
		fmt.Printf("Doing something with %s\n", file)
		if _, err := os.Stat(file); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "File %s does not exist.\n")
			continue
		}
		for _, idxFile := range idx.Objects {
			if idxFile.PathName == file {
				obj, err := GetObject(repo, idxFile.Sha1)
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
