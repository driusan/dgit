package cmd

import (
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func MkTree(c *git.Client, args []string) error {
	flags := newFlagSet("mktree")
	opts := git.MkTreeOptions{}

	flags.BoolVar(&opts.NilTerminate, "z", false, "Use nil to terminate lines")
	flags.BoolVar(&opts.AllowMissing, "missing", false, "Allow missing objects in the tree")
	flags.BoolVar(&opts.Batch, "batch", false, "Build more than one tree, separated by blank lines")

	flags.Parse(args)

	if opts.Batch {
		return git.MkTreeBatch(c, opts, os.Stdin, os.Stdout)
	}
	tree, err := git.MkTree(c, opts, os.Stdin)
	if err != nil {
		return err
	}
	fmt.Println(tree)
	return nil
}
