package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func SymbolicRef(c *git.Client, args []string) (git.RefSpec, error) {
	flags := flag.NewFlagSet("symbolic-ref", flag.ExitOnError)
	flags.SetOutput(os.Stdout)
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(os.Stdout, "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.SymbolicRefOptions{}

	reason := flags.String("m", "", "Reason to record in reflog for updating the reference")

	delete := flags.Bool("delete", false, "Delete the reference")
	d := flags.Bool("d", false, "Alias of --delete")

	quiet := flags.Bool("quiet", false, "Do not print an error if <name> is not a detached head")
	q := flags.Bool("q", false, "Alias of --quiet")

	flags.BoolVar(&opts.Short, "short", false, "Try to shorten the names of a symbolic ref")

	opts.Delete = *delete || *d
	opts.Quiet = *quiet || *q

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
	flag.Usage()
	return "", fmt.Errorf("Invalid usage")
}
