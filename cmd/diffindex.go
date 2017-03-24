package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

func DiffIndex(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("diff-index", flag.ExitOnError)

	options := git.DiffIndexOptions{}
	flags.BoolVar(&options.Cached, "cached", false, "Do not compare the filesystem, only the index")

	args, err := parseCommonDiffFlags(c, &options.DiffCommonOptions, false, flags, args)
	if err != nil {
		return err
	}

	if len(args) < 1 {
		return fmt.Errorf("Must provide a treeish to git diff-index")
	}

	treeish, err := git.RevParseCommit(c, &git.RevParseOptions{}, args[0])
	if err != nil {
		return err
	}
	diffs, err := git.DiffIndex(c, options, treeish, args[1:])
	if err != nil {
		return err
	}

	printDiffs(c, options.DiffCommonOptions, diffs)
	return nil
}
