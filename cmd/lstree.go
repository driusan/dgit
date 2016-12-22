package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/go-git/git"
)

func LsTree(c *git.Client, args []string) error {
	var treeonly, recursepaths, showtrees, endnil bool
	//var showlong1, showlong2 bool
	var nameonly, namestatus bool
	//flag.StringVar(&t, "t", "blob", "-t object type")
	flag.BoolVar(&treeonly, "d", true, "Show only the named tree, not its children")
	flag.BoolVar(&recursepaths, "r", false, "Recurse into sub-trees")
	flag.BoolVar(&showtrees, "t", false, "Show trees even when recursing into them")
	flag.BoolVar(&endnil, "z", false, "\\0 line termination on output")

	flag.BoolVar(&nameonly, "name-only", false, "Only show the names of the files")
	flag.BoolVar(&namestatus, "name-status", false, "Only show the names of the files")

	nameonly = nameonly || namestatus
	fakeargs := []string{"git-ls-tree"}
	os.Args = append(fakeargs, args...)

	flag.Parse()
	args = flag.Args()
	if len(args) < 1 {
		flag.Usage()
		return fmt.Errorf("Missing tree")
	}

	commitId, err := RevParse(c, []string{args[0]})
	if err != nil {
		return err
	}

	tree, err := git.ExpandTreeIntoIndexesById(c, commitId[0].Id.String(), recursepaths, showtrees)
	if err != nil {
		return err
	}
	for _, entry := range tree {
		var lineend string
		if endnil {
			lineend = "\000"
		} else {
			lineend = "\n"
		}
		if !nameonly {
			fmt.Printf("%0.6o %s %s\t%s%s", entry.Mode, entry.Sha1.Type(c), entry.Sha1, entry.PathName, lineend)
		} else {
			fmt.Printf("%s%s", entry.PathName, lineend)
		}
	}
	return nil
}
