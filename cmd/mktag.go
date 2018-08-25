package cmd

import (
	"flag"
	"fmt"
	"os"

	"../git"
)

// Implements the git mktag command line parsing.
func Mktag(c *git.Client, args []string) (git.Sha1, error) {
	flags := flag.NewFlagSet("tag", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	flags.Parse(args)
	if len(flags.Args()) > 0 {
		// There are no options for mktag
		flags.Usage()
		os.Exit(1)
	}
	return git.Mktag(c, os.Stdin)
}
