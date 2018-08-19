package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

func ShowRef(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("show-ref", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	//flags.Parse(args)
	return nil
}
