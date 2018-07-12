package cmd

import (
	"flag"
        "os"

	"github.com/driusan/dgit/git"
)

func DiffFiles(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("diff-files", flag.ExitOnError)
        flags.SetOutput(os.Stdout)
	options := git.DiffFilesOptions{}
	args, err := parseCommonDiffFlags(c, &options.DiffCommonOptions, false, flags, args)
	files := make([]git.File, len(args), len(args))
	for i := range args {
		files[i] = git.File(args[i])
	}
	diffs, err := git.DiffFiles(c, options, files)
	if err != nil {
		return err
	}
	return printDiffs(c, options.DiffCommonOptions, diffs)
}
