package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func RevList(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("rev-list", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.RevListOptions{}
	flags.BoolVar(&opts.Objects, "objects", false, "include non-commit objects in output")
	flags.BoolVar(&opts.Quiet, "quiet", false, "prevent printing of revisions")
	flags.BoolVar(&opts.VerifyObjects, "verify-objects", false, "verify objects instead of printing them")
	flags.BoolVar(&opts.All, "all", false, "pretend as if all refs were passed on the command line")

	flags.Parse(args)
	args = flags.Args()
	if opts.VerifyObjects {
		opts.Objects = true
	}
	// First get a map of excluded commitIDs
	var excludes []git.Commitish
	var includes []git.Commitish
	for _, rev := range args {
		if rev == "" {
			continue
		}
		if rev[0] == '^' && len(rev) > 1 {
			commits, _, err := RevParse(c, []string{rev[1:]})
			if err != nil {
				return fmt.Errorf("%s:%v", rev, err)
			}
			for _, cmt := range commits {
				excludes = append(excludes, cmt)
			}
		} else {
			commits, _, err := RevParse(c, []string{rev})
			if err != nil {
				return fmt.Errorf("%s:%v", rev, err)
			}
			for _, cmt := range commits {
				includes = append(includes, cmt)
			}
		}
	}
	_, err := git.RevList(c, opts, os.Stdout, includes, excludes)
	return err
}
