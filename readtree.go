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
	os.Args = append([]string{"git read-tree"}, args...)
	indexName := flag.String("index-output", "index", "Name of the file to read the tree into")
	flag.Parse()
	args = flag.Args()
	fmt.Printf("%v into %v\n", args, *indexName)
	if len(args) != 1 {
		flag.Usage()
		return fmt.Errorf("Invalid usage")
	}

	commits, err := RevParse(c, args)
	if err != nil {
		return err
	}
	c.ResetIndex(commits[0], *indexName)
	return nil
}
