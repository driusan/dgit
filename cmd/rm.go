package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Rm(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("rm", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	var opts git.RmOptions
	flags.BoolVar(&opts.Force, "force", false, "Override the up to date check")
	flags.BoolVar(&opts.Force, "f", false, "Alias of force")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Do not actually remove file or update index")
	flags.BoolVar(&opts.DryRun, "n", false, "Alias of dry-run")
	flags.BoolVar(&opts.Recursive, "r", false, "Allow recursive removal of directories")
	flags.BoolVar(&opts.Cached, "cached", false, "Only remove from the index, not the filesystem")
	flags.BoolVar(&opts.IgnoreUnmatched, "ignore-unmatched", false, "Exit with a success even if no files matched")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Do not output the name of removed files")
	flags.BoolVar(&opts.Quiet, "q", false, "Alias of quiet")

	flags.Parse(args)
	sfiles := flags.Args()
	if len(sfiles) < 1 {
		flags.Usage()
		os.Exit(1)
	}
	files := make([]git.File, 0, len(sfiles))
	for _, f := range sfiles {
		files = append(files, git.File(f))
	}
	return git.Rm(c, opts, files)
}
