package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Pull(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("pull", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	fetchopts := git.FetchOptions{}

	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"all", "a", "append", "unshallow", "update-shallow", "dry-run", "f", "force", "k", "keep", "multiple", "p", "prune", "P", "prune-tags", "n", "no-tags", "t", "tags", "no-recurse-submodules", "u", "update-head-ok", "q", "quiet", "v", "verbose", "progress", "4", "ipv4", "ipv6"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"depth", "deepend", "shallow-since", "shallow-exclude", "refmap", "recurse-submodules", "j", "jobs", "submodule-prefix", "recurse-submodules-default", "upload-pack", "o", "server-option"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

        mergeopts := git.MergeOptions{}

        flags.BoolVar(&mergeopts.FastForwardOnly, "ff-only", false, "Only allow fast-forward merges")
        flags.BoolVar(&mergeopts.NoFastForward, "no-ff", false, "Create a merge commit even when it's a fast-forward merge.")

	flags.Parse(args)

	var repository string
        var refspec []string
	if flags.NArg() < 1 {
		repository = "origin" // FIXME origin is the default unless the current branch unless there is an upstream branch configured for the current branch
                refspec = []string{"origin/master"}
	} else if flags.NArg() == 1 {
		repository = flags.Arg(0)
                refspec = []string{repository + "/master"}
        } else if flags.NArg() >= 2 {
                repository = flag.Arg(0)
                refspec = flag.Args()[1:]
	} else {
		flags.Usage()
		os.Exit(1)
	}

	err := git.Fetch(c, fetchopts, repository)
        if err != nil {
                return err
        }

        others := make([]git.Commitish, 0, len(refspec))
        for _, name := range refspec {
                c, err := git.RevParseCommitish(c, &git.RevParseOptions{}, name)
                if err != nil {
                        return err
                }
                others = append(others, c)
        }

        err = git.Merge(c, mergeopts, others)
        if err != nil {
                return err
        }

        return nil
}
