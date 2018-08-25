package cmd

import (
	"flag"
	"fmt"

	"../git"
)

func Show(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("show", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.ShowOptions{}
	flags.Var(newAliasedStringValue((*string)(&opts.Format), ""), "format", "Print the contents of commit logs in a specified format")
	flags.Var(newAliasedStringValue((*string)(&opts.Format), ""), "pretty", "Alias for --format")
	flags.Parse(args)

	objects := flags.Args()
	return git.Show(c, opts, objects)
}
