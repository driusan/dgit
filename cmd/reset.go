package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Reset(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("reset", flag.ExitOnError)
	flags.SetOutput(os.Stdout)
	flags.Usage = func() {
		flag.Usage()
		fmt.Printf("\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.ResetOptions{}

	flags.BoolVar(&opts.Quiet, "q", false, "Only report errors")

	flags.BoolVar(&opts.Soft, "soft", false, "Do not touch the index or working tree, but reset the head to commit")
	flags.BoolVar(&opts.Mixed, "mixed", false, "Reset the index but not the working tree")
	flags.BoolVar(&opts.Hard, "hard", false, "Reset both the index and working tree.")
	flags.BoolVar(&opts.Merge, "merge", false, "Reset the index and update working tree for files different between <commit> and HEAD, but not those different between index and working tree")
	flags.BoolVar(&opts.Keep, "keep", false, "Like --merge, but abort if any files have local changes instead of leaving them")

	flags.Parse(args)

	args = flags.Args()
	files := make([]git.File, 0, len(args))
	for _, f := range args {
		files = append(files, git.File(f))
	}
	return git.Reset(c, opts, files)
}
