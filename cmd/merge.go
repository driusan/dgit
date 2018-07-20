package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Merge(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("merge", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}
	options := git.MergeOptions{}

	flags.BoolVar(&options.FastForwardOnly, "ff-only", false, "Only allow fast-forward merges")
	flags.BoolVar(&options.NoFastForward, "no-ff", false, "Create a merge commit even when it's a fast-forward merge.")
	abort := flag.Bool("abort", false, "Abort an in-progress merge")
	flags.Parse(args)

	if *abort {
		return git.MergeAbort(c, options)
	}
	merges := flags.Args()
	if len(merges) < 1 {
		flags.Usage()
		os.Exit(2)
	}

	others := make([]git.Commitish, 0, len(merges))
	for _, name := range merges {
		c, err := git.RevParseCommitish(c, &git.RevParseOptions{}, name)
		if err != nil {
			return err
		}
		others = append(others, c)
	}
	return git.Merge(c, options, others)
}
