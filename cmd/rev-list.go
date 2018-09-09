package cmd

import (
	"flag"
	"fmt"

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
	flags.Parse(args)
	args = flags.Args()

	// First get a map of excluded commitIDs
	var excludes []git.Commitish
	var includes []git.Commitish
	for _, rev := range args {
		if rev == "" {
			continue
		}
		if rev[0] == '^' && len(rev) > 1 {
			commits, err := RevParse(c, []string{rev[1:]})
			if err != nil {
				return fmt.Errorf("%s:%v", rev, err)
			}
			for _, cmt := range commits {
				excludes = append(excludes, cmt)
			}
		} else {
			commits, err := RevParse(c, []string{rev})
			if err != nil {
				return fmt.Errorf("%s:%v", rev, err)
			}
			for _, cmt := range commits {
				includes = append(includes, cmt)
			}
		}
	}
	return git.RevListCallback(c, opts, includes, excludes, func(s git.Sha1) error {
		if !opts.Quiet {
			fmt.Println(s)
		}
		return nil
	})
}
