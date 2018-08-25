package cmd

import (
	"flag"
	"fmt"
	"os"

	"../git"
)

func Branch(c *git.Client, args []string) {
	flags := flag.NewFlagSet("branch", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"d", "delete", "D", "create-reflog", "f", "force", "m", "move", "M", "c", "copy", "C", "no-color", "i", "ignore-case", "no-column", "r", "remotes", "a", "all", "v", "vv", "verbose", "q", "quiet", "no-abbrev", "no-track", "unset-upstream", "edit-description"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"color", "abbrev", "column", "sort", "no-merged", "contains", "no-contains", "points-at", "format", "set-upstream-to", "u"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	flags.Parse(args)

	switch flags.NArg() {
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
		if err := c.CreateBranch(flags.Arg(0), b); err != nil {
			fmt.Fprintf(os.Stderr, "Could not create branch (%v): %v\n", flags.Arg(0), err)
		}
	default:
		flag.Usage()
		os.Exit(2)
	}

}
