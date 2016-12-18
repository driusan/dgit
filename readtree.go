package main

import (
	"flag"
	"fmt"
	"os"
)

// Resets the index to the Treeish tree and save the results in
// the file named indexname
func (c *Client) ResetIndex(tree Treeish, indexname string) error {
	// If the index doesn't exist, idx is a new index, so ignore
	// the path error that ReadIndex is returning
	idx, _ := c.GitDir.ReadIndex()
	idx.ResetIndex(c, tree)

	f, err := c.GitDir.Create(File(indexname))
	if err != nil {
		return err
	}
	defer f.Close()
	return idx.WriteIndex(f)
}

func ReadTree(c *Client, args []string) error {
	flags := flag.NewFlagSet("read-tree", flag.ExitOnError)
	indexName := flags.String("index-output", "index", "Name of the file to read the tree into")
	flags.Parse(args)
	args = flags.Args()
	fmt.Printf("%v into %v\n", args, *indexName)
	flags.Usage = func() {
		subcommandUsage = fmt.Sprintf("%s [global options] %s [options] <treeish>\n", os.Args[0], subcommand)

		flag.Usage()
		fmt.Fprintf(os.Stderr, "\nread-tree options:\n\n")
		flags.PrintDefaults()
	}
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
