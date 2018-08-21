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

	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"q", "quiet", "verify", "head", "d", "dereference", "tags", "heads"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"s", "hash", "abbrev"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	flags.Parse(args)
	return nil
}
