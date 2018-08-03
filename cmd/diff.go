package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/driusan/dgit/git"
)

func Diff(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("diff", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}
	options := git.DiffOptions{}

	var staged bool
	flags.BoolVar(&staged, "staged", false, "Synonym for cached")
	var cached bool
	flags.BoolVar(&cached, "cached", false, "Display changes staged for commit")
	flags.BoolVar(&options.NoIndex, "no-index", false, "Use diff to display difference between files on the filesystem")

	args, err := parseCommonDiffFlags(c, &options.DiffCommonOptions, true, flags, args)

	if staged || cached {
		options.Staged = true
	}

	files := make([]git.File, len(args), len(args))
	for i := range args {
		files[i] = git.File(args[i])
	}
	diffs, err := git.Diff(c, options, files)
	if err != nil {
		if options.NoIndex {
			switch e := err.(type) {
			case *exec.ExitError:
				if e.Exited() {
					// We don't want the exit status printed by returning it,
					// so we just call os.Exit.
					// (If there were no diffs err will be nil)
					os.Exit(1)
				}

			default:
			}
		}
		return err
	}
	return printDiffs(c, options.DiffCommonOptions, diffs)
}
