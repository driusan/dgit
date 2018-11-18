package cmd

import (
	"flag"
	"fmt"
	"github.com/driusan/dgit/git"
	"os"
)

func Branch(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("branch", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.BranchOptions{}

	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"d", "delete", "D", "create-reflog", "f", "force", "m", "move", "M", "c", "copy", "C", "no-color", "i", "ignore-case", "no-column", "r", "remotes", "v", "vv", "verbose", "no-abbrev", "no-track", "unset-upstream", "edit-description"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"color", "abbrev", "column", "sort", "no-merged", "contains", "no-contains", "points-at", "format", "set-upstream-to", "u"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	flags.BoolVar(&opts.All, "all", false, "Show remote branches too")
	flags.BoolVar(&opts.All, "a", false, "Alias of --all")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Do not print branches")
	flags.BoolVar(&opts.Quiet, "q", false, "Alias of --quiet")
	flags.Parse(args)

	switch flags.NArg() {
	case 0:
		_, err := git.BranchList(c, os.Stdout, opts, nil)
		return err
	case 1:
		headref, err := git.SymbolicRefGet(c, git.SymbolicRefOptions{}, "HEAD")
		if err != nil {
			return err
		}
		b := git.Branch(headref)
		if err := c.CreateBranch(flags.Arg(0), b); err != nil {
			fmt.Fprintf(os.Stderr, "Could not create branch (%v): %v\n", flags.Arg(0), err)
		}
	case 2:
		startpoint, err := git.RevParseCommitish(c, &git.RevParseOptions{}, flags.Arg(1))
		if err != nil {
			return err
		}
		if err := c.CreateBranch(flags.Arg(0), startpoint); err != nil {
			fmt.Fprintf(os.Stderr, "Could not create branch (%v): %v\n", flags.Arg(0), err)
		}
	default:
		flag.Usage()
		os.Exit(2)
	}
	return nil

}
