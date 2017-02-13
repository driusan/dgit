package cmd

import (
	"flag"

	"github.com/driusan/dgit/git"
)

func Diff(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("diff", flag.ExitOnError)
	options := git.DiffOptions{}
	flags.BoolVar(&options.Staged, "staged", false, "Display changes staged for commit")

	args, err := parseCommonDiffFlags(c, &options.DiffCommonOptions, true, flags, args)

	diffs, err := git.Diff(c, options, args)
	if err != nil {
		return err
	}

	printDiffs(c, options.DiffCommonOptions, diffs)
	return err
}
