package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Add(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("add", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.AddOptions{}

	flags.BoolVar(&opts.Verbose, "verbose", false, "Be verbose about what's being added")
	flags.BoolVar(&opts.Verbose, "v", false, "Alias of --verbose")

	flags.BoolVar(&opts.DryRun, "dry-run", false, "Do not update the index, only show what would happen")
	flags.BoolVar(&opts.DryRun, "n", false, "Alias of --dry-run")

	flags.BoolVar(&opts.Force, "force", false, "Allow adding ignored files")
	flags.BoolVar(&opts.Force, "f", false, "Alias of --force")

	flags.BoolVar(&opts.Interactive, "interactive", false, "Interactively stage changes from the working directory")
	flags.BoolVar(&opts.Interactive, "i", false, "Alias of --interactive")

	flags.BoolVar(&opts.Patch, "patch", false, "Interactively stage hunks from the working directory")
	flags.BoolVar(&opts.Patch, "p", false, "Alias of --patch")

	flags.BoolVar(&opts.Edit, "edit", false, "Open the diff and allow it to be edited before staging")
	flags.BoolVar(&opts.Edit, "e", false, "Alias of --edit")

	flags.BoolVar(&opts.Update, "update", false, "Only update files that already exist in the index")
	flags.BoolVar(&opts.Update, "u", false, "Alias of --update")

	flags.BoolVar(&opts.All, "all", false, "Update, add, or remove all files from the index")
	flags.BoolVar(&opts.All, "A", false, "Alias of --all")
	flags.BoolVar(&opts.All, "no-ignore-removal", false, "Alias of --all")

	flags.BoolVar(&opts.IgnoreRemoval, "no-all", false, "Do not remove files that have been removed from the working tree")
	flags.BoolVar(&opts.IgnoreRemoval, "ignore-removal", false, "Alias of --no-all")

	flags.BoolVar(&opts.IntentToAdd, "intent-to-add", false, "Only record the fact that the file will be added later, do not add it")
	flags.BoolVar(&opts.IntentToAdd, "N", false, "Alias of --intent-to-add")

	flags.BoolVar(&opts.Refresh, "refresh", false, "Don't add files, only refresh their stat information")
	flags.BoolVar(&opts.IgnoreErrors, "ignore-errors", false, "If some files could not be added, do not abort, but continue with the others.")
	flags.BoolVar(&opts.IgnoreMissing, "ignore-missing", false, "If some files could not be added, do not abort, but continue with the others.")

	flags.BoolVar(&opts.NoWarnEmbeddedRepo, "no-warn-embedded-repo", false, "No-op, submodules are not supported..")

	chmod := flags.String("chmod", "", "Override the executable bit of files")

	flags.Parse(args)

	switch *chmod {
	case "":
		opts.Chmod.Modify = false
	case "+x":
		opts.Chmod.Modify = true
		opts.Chmod.Value = true
	case "-x":
		opts.Chmod.Modify = true
		opts.Chmod.Value = false
	default:
		fmt.Fprintf(flag.CommandLine.Output(), "Invalid value for --chmod option. Must be +x or -x\n")
		flags.Usage()
		os.Exit(2)
	}

	remaining := flags.Args()
	files := make([]git.File, len(remaining), len(remaining))
	for i := range remaining {
		files[i] = git.File(remaining[i])
	}
	_, err := git.Add(c, opts, files)
	return err
}
