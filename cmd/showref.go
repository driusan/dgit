package cmd

import (
	"flag"
	"fmt"
	"os"

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

	opts := git.ShowRefOptions{}
	flags.BoolVar(&opts.IncludeHead, "head", false, "Include the HEAD reference")
	flags.BoolVar(&opts.Heads, "heads", false, "Show only heads")
	flags.BoolVar(&opts.Tags, "tags", false, "Show only tags")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Do not print matching refs")
	flags.BoolVar(&opts.Quiet, "q", false, "alias of --q")
	flags.BoolVar(&opts.Verify, "verify", false, "verify the existence of an exact ref")
	flags.BoolVar(&opts.Dereference, "dereference", false, "dereference annotated tags")
	flags.BoolVar(&opts.Dereference, "d", false, "alias of -d")

	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, sf := range []string{"s", "hash", "abbrev"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	flags.Parse(args)
	refs, err := git.ShowRef(c, opts, flags.Args())
	if err != nil {
		return err
	}
	if !opts.Quiet {
		for _, ref := range refs {
			fmt.Println(ref)
		}
	}
	if len(refs) == 0 {
		os.Exit(1)
	}
	return nil
}
