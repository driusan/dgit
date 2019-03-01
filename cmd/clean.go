package cmd

import (
	"flag"
	"fmt"
	"github.com/driusan/dgit/git"
)

func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions: \n")
		flags.PrintDefaults()
	}
	return flags
}

func Clean(c *git.Client, args []string) error {
	flags := newFlagSet("clean")
	opts := git.CleanOptions{}
	flags.BoolVar(&opts.Directory, "d", false, "Remove untracked directories in addition to files")
	flags.BoolVar(&opts.Force, "force", false, "Do deletion even if clean.requireForce is not false")
	flags.BoolVar(&opts.Force, "f", false, "Alias of --force")
	flags.BoolVar(&opts.Interactive, "i", false, "Not implemented")
	flags.BoolVar(&opts.Interactive, "interactive", false, "Not implemented")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Do not do deletion, just show what would be done")
	flags.BoolVar(&opts.DryRun, "n", false, "Alias of --dry-run")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Do not print file names as they are deleted")
	flags.BoolVar(&opts.Quiet, "q", false, "Alias of --quiet")

	flags.Var(NewMultiStringValue(&opts.ExcludePatterns), "exclude", "Add pattern to standard exclude patterns")
	flags.Var(NewMultiStringValue(&opts.ExcludePatterns), "e", "Alias of --exclude")
	flags.BoolVar(&opts.NoStandardExclude, "x", false, "Do not use standard .gitignore and .git/info/exclude patterns")
	flags.BoolVar(&opts.OnlyExcluded, "X", false, "Only remove files ignored by git")

	flags.Parse(args)

	config, err := git.LoadLocalConfig(c)
	if err != nil {
		return err
	}
	requireforce, _ := config.GetConfig("clean.requireforce")
	if requireforce != "false" && !(opts.DryRun || opts.Force) {
		return fmt.Errorf("clean.requireforce defaults to true and -f or -n not set")
	}
	paths := flags.Args()
	var files []git.File
	for _, p := range paths {
		files = append(files, git.File(p))
	}
	return git.Clean(c, opts, files)
}
