package cmd

import (
	"flag"

	"github.com/driusan/dgit/git"
)

func Diff(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("diff", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	options := git.DiffOptions{}
	flags.BoolVar(&options.Staged, "staged", false, "Display changes staged for commit")

	args, err := parseCommonDiffFlags(c, &options.DiffCommonOptions, true, flags, args)

	files := make([]git.File, len(args), len(args))
	for i := range args {
		files[i] = git.File(args[i])
	}
	diffs, err := git.Diff(c, options, files)
	if err != nil {
		return err
	}
	return printDiffs(c, options.DiffCommonOptions, diffs)
}
