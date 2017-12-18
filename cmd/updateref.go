package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func UpdateRef(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("update-ref", flag.ExitOnError)
	flags.Usage = func() {
		flag.Usage()
		flags.PrintDefaults()
	}

	opts := git.UpdateRefOptions{}

	reason := flags.String("m", "", "Reason to record in reflog for updating the reference")
	flags.BoolVar(&opts.Delete, "d", false, "Delete the reference after verifying oldvalue")
	flags.BoolVar(&opts.NoDeref, "no-deref", false, "Do not dereference symbolic references")
	flags.BoolVar(&opts.NoDeref, "create-reflog", false, "Create a reflog if it doesn't exist")

	stdin := flags.Bool("stdin", false, "Read references from stdin in batch mode")
	flags.BoolVar(&opts.NullTerminate, "z", false, `Use \0 instead of \n to terminate lines in batch mode`)

	flags.Parse(args)
	vals := flags.Args()

	if *stdin {
		opts.Stdin = os.Stdin
	}

	switch len(vals) {
	case 1:
		if opts.Delete {
			return git.UpdateRef(c, opts, vals[0], git.CommitID{}, *reason)
		}
		// 1 options with -d is meaningless, fall through to printusage
	case 2:
		if opts.Delete {
			oldval, err := git.CommitIDFromString(vals[1])
			if err != nil {
				return err
			}
			opts.OldValue = oldval
			return git.UpdateRef(c, opts, vals[0], git.CommitID{}, *reason)

		}
		cmt, err := git.RevParseCommit(c, &git.RevParseOptions{}, vals[1])
		if err != nil {
			return fmt.Errorf("Invalid commit %s", vals[1])
		}
		return git.UpdateRef(c, opts, vals[0], cmt, *reason)
	case 3:
		if opts.Delete {
			// There is no delete variaton with 3 parameters, abort.
			break
		}
		cmt, err := git.RevParseCommit(c, &git.RevParseOptions{}, vals[1])
		if err != nil {
			return fmt.Errorf("Invalid commit %s", vals[1])
		}
		oldval, err := git.CommitIDFromString(vals[2])
		if err != nil {
			return err
		}
		opts.OldValue = oldval

		return git.UpdateRef(c, opts, vals[0], cmt, *reason)
	}
	flags.Usage()
	return fmt.Errorf("Invalid usage")
}
