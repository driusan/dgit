package cmd

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/driusan/dgit/git"
)

func Log(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("log", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	flags.Var(newNotimplBoolValue(), "follow", "Not implemented")
	flags.Var(newNotimplBoolValue(), "no-decorate", "Not implemented")
	flags.Var(newNotimplStringValue(), "decorate", "Not implemented")
	flags.Var(newNotimplStringValue(), "decorate-refs", "Not implemented")
	flags.Var(newNotimplStringValue(), "decorate-refs-exclude", "Not implemented")
	flags.Var(newNotimplBoolValue(), "source", "Not implemented")
	flags.Var(newNotimplBoolValue(), "use-mailmap", "Not implemented")
	flags.Var(newNotimplBoolValue(), "full-diff", "Not implemented")
	flags.Var(newNotimplStringValue(), "log-size", "Not implemented")
	flags.Var(newNotimplStringValue(), "L", "Not implemented")
	maxCount := -1
	flags.IntVar(&maxCount, "n", -1, "Limit the number of commits.")
	flags.IntVar(&maxCount, "max-count", -1, "Alias for -n")
	format := "medium" // The default
	flags.StringVar(&format, "format", "medium", "Pretty print the commit logs")

	adjustedArgs := []string{}
	for _, a := range args {
		if strings.HasPrefix(a, "-n") && a != "-n" {
			adjustedArgs = append(adjustedArgs, "-n", a[2:])
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			if _, err := strconv.Atoi(a[1:]); err == nil {
				adjustedArgs = append(adjustedArgs, "-n", a[1:])
				continue
			}
		}
		adjustedArgs = append(adjustedArgs, a)
	}

	flags.Parse(adjustedArgs)

	if flags.NArg() > 1 {
		fmt.Fprintf(flag.CommandLine.Output(), "Paths are not yet implemented, just the revision")
		flags.Usage()
		os.Exit(2)
	}

	var commit git.Commitish
	var err error
	if flags.NArg() == 0 {
		commit, err = git.RevParseCommitish(c, &git.RevParseOptions{}, "HEAD")
	} else {
		commit, err = git.RevParseCommitish(c, &git.RevParseOptions{}, flags.Arg(0))
	}
	if err != nil {
		return err
	}

	opts := git.RevListOptions{Quiet: true}
	if maxCount >= 0 {
		mc := uint(maxCount)
		opts.MaxCount = &mc
	}

	printCommit, err := git.GetCommitPrinter(c, format)
	if err != nil {
		return err
	}

	err = git.RevListCallback(c, opts, []git.Commitish{commit}, nil, func(s git.Sha1) error {
		return printCommit(c, git.CommitID(s), format)
	})
	return err
}
