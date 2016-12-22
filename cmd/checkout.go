package cmd

import (
	"fmt"
	"github.com/driusan/go-git/git"
	"io/ioutil"
	"os"
)

// Implements the git checkout command.
//
// BUG(driusan): This needs to be completely rewritten in terms of checkout-index
// and ReadTree, now that they're implemented. CheckoutIndex is much safer and
// more in line with the git client, while this is a hack.
func Checkout(c *git.Client, args []string) {
	switch len(args) {
	case 0:
		fmt.Fprintf(os.Stderr, "Missing argument for checkout")
		return
	}

	idx, _ := c.GitDir.ReadIndex()
	for _, file := range args {
		f := git.File(file)
		if !f.Exists() {
			continue
		}
		ipath, err := f.IndexPath(c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			continue
		}

		for _, idxFile := range idx.Objects {
			if idxFile.PathName == ipath {
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
