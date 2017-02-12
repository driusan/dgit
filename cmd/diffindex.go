package cmd

import (
	"flag"

	"github.com/driusan/dgit/git"
)

func DiffIndex(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("diff-index", flag.ExitOnError)

	options := git.DiffIndexOptions{}
	flags.BoolVar(&options.Cached, "cached", false, "Do not compare the filesystem, only the index")

	args, err := parseCommonDiffFlags(c, &options.DiffCommonOptions, flags, args)

	treeish, err := git.RevParseCommit(c, &git.RevParseOptions{}, args[0])
	if err != nil {
		return err
	}
	diffs, err := git.DiffIndex(c, &options, treeish, args[1:])
	if err != nil {
		return err
	}

	printDiffs(c, options.DiffCommonOptions, diffs)
	return nil
}
