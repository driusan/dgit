package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func CheckIgnore(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("check-ignore", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.CheckIgnoreOptions{}

	for _, v := range []string{"q", "quiet", "v", "stdin", "n", "non-matching", "no-index"} {
		flags.Var(newNotimplBoolValue(), v, "Not implemented")
	}

	flags.Parse(args)
	args = flags.Args()

	if len(args) < 1 {
		fmt.Fprintf(flag.CommandLine.Output(), "Check ignore requires at least one path")
		flags.Usage()
		os.Exit(2)
	}

	paths := make([]git.File, 0, len(args))

	for _, p := range args {
		paths = append(paths, git.File(p))
	}

	patterns, err := git.CheckIgnore(c, opts, paths)

	if err != nil {
		return err
	}

	exitCode := 1
	for idx, pattern := range patterns {
		if pattern != "" {
			fmt.Printf("%s\n", paths[idx].String())
			exitCode = 0
		}
	}

	os.Exit(exitCode)
	return nil
}
