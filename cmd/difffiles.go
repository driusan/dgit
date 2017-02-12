package cmd

import (
	"flag"

	"github.com/driusan/dgit/git"
)

func DiffFiles(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("diff-files", flag.ExitOnError)
	options := git.DiffFilesOptions{}
	args, err := parseCommonDiffFlags(c, &options.DiffCommonOptions, flags, args)

	diffs, err := git.DiffFiles(c, &options, args)
	if err != nil {
		return err
	}

	printDiffs(c, options.DiffCommonOptions, diffs)
	return err
}
