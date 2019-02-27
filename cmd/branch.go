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
	for _, bf := range []string{"create-reflog", "f", "force", "M", "c", "copy", "C", "no-color", "i", "ignore-case", "no-column", "r", "remotes", "v", "vv", "verbose", "no-abbrev", "no-track", "unset-upstream", "edit-description"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"color", "abbrev", "column", "sort", "no-merged", "contains", "no-contains", "points-at", "format", "set-upstream-to", "u"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	flags.BoolVar(&opts.All, "all", false, "Show remote branches too")
	flags.BoolVar(&opts.All, "a", false, "Alias of --all")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Do not print branches")
	flags.BoolVar(&opts.Quiet, "q", false, "Alias of --quiet")
	flags.BoolVar(&opts.Move, "m", false, "Move/rename a branch")
	flags.BoolVar(&opts.Move, "move", false, "Alias of -m")
	flags.BoolVar(&opts.Delete, "d", false, "Delete a branch")
	flags.BoolVar(&opts.Delete, "delete", false, "Alias of -d")
	flags.BoolVar(&opts.Delete, "D", false, "Alias of -d") // This wil no longer be a simple alias once we have --force
	flags.Parse(args)

	if opts.Delete {
		for idx := range flags.Args() {
			branch, err := git.GetBranch(c, flags.Arg(idx))
			if err != nil {
				return err
			}
			if err = branch.DeleteBranch(c); err != nil {
				return err
			}
		}
		return nil
	}

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
			return err
		}
		if opts.Move {
			newbranch, err := git.GetBranch(c, flags.Arg(0))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not get branch (%v): %v\n", flags.Arg(0), err)
				return err
			}
			refmsg := fmt.Sprintf("Branch: renamed %s to %s", b.String(), flags.Arg(0))
			if err = git.SymbolicRefUpdate(c, git.SymbolicRefOptions{}, "HEAD", git.RefSpec(newbranch), refmsg); err != nil {
				fmt.Fprintf(os.Stderr, "Could not update symbolic ref to branch (%v): %v\n", flags.Arg(0), err)
				return err
			}
			if err = b.DeleteBranch(c); err != nil {
				fmt.Fprintf(os.Stderr, "Could not delete original branch (%v): %v\n", b, err)
				return err
			}
			// TODO move the reflog
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
