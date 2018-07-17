package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

func DiffIndex(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("diff-index", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

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

	files := make([]git.File, len(args)-1, len(args)-1)
	for i, f := range args[1:] {
		files[i] = git.File(f)
	}
	index, _ := c.GitDir.ReadIndex()
	diffs, err := git.DiffIndex(c, options, index, treeish, files)
	if err != nil {
		return err
	}
	return printDiffs(c, options.DiffCommonOptions, diffs)
}
