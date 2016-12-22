package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/go-git/git"
)

func ReadTree(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("read-tree", flag.ExitOnError)
	indexName := flags.String("index-output", "index", "Name of the file to read the tree into")
	flags.Parse(args)
	args = flags.Args()
	if len(args) != 1 {
		flags.Usage()
		return fmt.Errorf("Invalid usage")
	}

	commits, err := RevParse(c, args)
	if err != nil {
		return err
	}
	c.ResetIndex(commits[0], *indexName)
	return nil
}
