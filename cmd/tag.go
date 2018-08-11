package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

// Implements the git tag command line parsing.
func Tag(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("tag", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}
	options := git.TagOptions{}

	flags.BoolVar(&options.Force, "force", false, "Replace an existing tag if it exists")
	flags.BoolVar(&options.Force, "f", false, "Alias of --force")

	flags.Parse(args)
	tagnames := flags.Args()
	switch len(tagnames) {
	case 0:
		return fmt.Errorf("Listing tags not implemented")
	case 1:
		return git.TagCommit(c, options, tagnames[0], nil)
	case 2:
		commit, err := git.RevParseCommitish(c, &git.RevParseOptions{}, tagnames[1])
		if err != nil {
			return err
		}
		return git.TagCommit(c, options, tagnames[0], commit)
	default:
		return fmt.Errorf("Invalid tag usage")
	}
}
