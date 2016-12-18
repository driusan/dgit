package main

import (
	"fmt"
	"os"
)

func Branch(c *Client, args []string) {
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
			fmt.Println(b)
		}
	case 1:
		head, err := c.GetSymbolicRefCommit(getSymbolicRef(c, "HEAD"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create branch (%v): %v\n", head, err)
			return
		}
		if err := c.CreateBranch(args[0], head); err != nil {
			fmt.Fprintf(os.Stderr, "Could not create branch (%v): %v\n", head, err)
		}
	default:
		fmt.Fprintln(os.Stderr, "Usage: go-git branch [branchname]")
	}

}
