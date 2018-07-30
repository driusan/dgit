package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Fetch(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("fetch", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.FetchOptions{}

	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"all", "a", "append", "unshallow", "update-shallow", "dry-run", "f", "force", "k", "keep", "multiple", "p", "prune", "P", "prune-tags", "n", "no-tags", "t", "tags", "no-recurse-submodules", "u", "update-head-ok", "q", "quiet", "v", "verbose", "progress", "4", "ipv4", "ipv6"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"depth", "deepend", "shallow-since", "shallow-exclude", "refmap", "recurse-submodules", "j", "jobs", "submodule-prefix", "recurse-submodules-default", "upload-pack", "o", "server-option"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	flags.Parse(args)

	var repository string
	if flags.NArg() < 1 {
		repository = "origin" // FIXME origin is the default unless the current branch unless there is an upstream branch configured for the current branch
	} else if flags.NArg() == 1 {
		repository = flags.Arg(0)
	} else {
		fmt.Fprintf(os.Stderr, "Group and multiple repositories is not currently implemented\n")
		flags.Usage()
		os.Exit(1)
	}

	return git.Fetch(c, opts, repository)
}
