package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

func Show(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("show", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "show [options] <commit>...\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.ShowOptions{}
	flags.StringVar(&opts.Pretty, "pretty", "", "Pretty-print the contents of commit logs in a specified format")
	flags.Parse(args)

	objects := flags.Args()
	return git.Show(c, opts, objects)
}
