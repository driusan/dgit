package cmd

import (
	"flag"
	"fmt"
	"os"

	"../git"
)

func SymbolicRef(c *git.Client, args []string) (git.RefSpec, error) {
	flags := flag.NewFlagSet("symbolic-ref", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.SymbolicRefOptions{}

	reason := flags.String("m", "", "Reason to record in reflog for updating the reference")

	flags.BoolVar(&opts.Delete, "delete", false, "Delete the reference")
	flags.BoolVar(&opts.Delete, "d", false, "Alias of --delete")

	flags.BoolVar(&opts.Quiet, "quiet", false, "Do not print an error if <name> is not a detached head")
	flags.BoolVar(&opts.Quiet, "q", false, "Alias of --quiet")

	flags.BoolVar(&opts.Short, "short", false, "Try to shorten the names of a symbolic ref")

	flags.Parse(args)
	vals := flags.Args()

	switch len(vals) {
	case 1:
		if opts.Delete {
			return "", git.SymbolicRefDelete(c, opts, git.SymbolicRef(vals[0]))
		} else {
			return git.SymbolicRefGet(c, opts, git.SymbolicRef(vals[0]))
		}
	case 2:
		return "", git.SymbolicRefUpdate(c, opts, git.SymbolicRef(vals[0]), git.RefSpec(vals[1]), *reason)
	}

	flags.Usage()
	os.Exit(2)

	return "", fmt.Errorf("Invalid usage")
}
