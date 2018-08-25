package cmd

import (
	"flag"
	"fmt"

	"../git"
)

func printTags(tags []string) {
	for _, t := range tags {
		fmt.Println(t)
	}
}

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
	flags.BoolVar(&options.List, "list", false, "List tags")
	flags.BoolVar(&options.List, "l", false, "Alias of --list")

	flags.Parse(args)
	tagnames := flags.Args()
	if options.List {
		tags, err := git.TagList(c, tagnames)
		if err != nil {
			return err
		}
		printTags(tags)
		return nil
	}
	switch len(tagnames) {
	case 0:
		tags, err := git.TagList(c, tagnames)
		if err != nil {
			return err
		}
		printTags(tags)
		return nil
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
