package cmd

import (
	"flag"
	"fmt"
	"github.com/driusan/dgit/git"
	"os"
)

func Branch(c *git.Client, args []string) {
	if len(args) == 1 && args[0] == "--help" {
		flag.Usage()
		os.Exit(0)
	}

	switch len(args) {
	case 0:
		branches, err := c.GetBranches()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not get list of branches.")
			return
		}
		head := c.GetHeadBranch()
		for _, b := range branches {
			if head == b {
				fmt.Print("* ")
			} else {
				fmt.Print("  ")
			}
			fmt.Println(b.BranchName())
		}
	case 1:
		headref, err := git.SymbolicRefGet(c, git.SymbolicRefOptions{}, "HEAD")
		if err != nil {
			return
		}
		b := git.Branch(headref)
		if err := c.CreateBranch(args[0], b); err != nil {
			fmt.Fprintf(os.Stderr, "Could not create branch (%v): %v\n", args[0], err)
		}
	default:
		flag.Usage()
		os.Exit(2)
	}

}
