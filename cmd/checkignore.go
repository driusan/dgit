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

	quiet := false
	flags.BoolVar(&quiet, "quiet", false, "Don't output anything, just set exit status. This is only valid with a single pathname.")
	flags.BoolVar(&quiet, "q", false, "Alias for --quiet")
	verbose := false
	flags.BoolVar(&verbose, "verbose", false, "Also output details about the matching pattern (if any) for each given pathname.")
	flags.BoolVar(&verbose, "v", false, "Alias for --verbose")

	for _, v := range []string{"stdin", "n", "non-matching", "no-index"} {
		flags.Var(newNotimplBoolValue(), v, "Not implemented")
	}

	flags.Parse(args)
	args = flags.Args()

	if len(args) < 1 {
		fmt.Fprintf(flag.CommandLine.Output(), "Check ignore requires at least one path.\n")
		flags.Usage()
		os.Exit(2)
	} else if quiet && len(args) != 1 {
		fmt.Fprintf(flag.CommandLine.Output(), "Quiet is only valid with one pathname.\n")
		flags.Usage()
		os.Exit(2)
	}

	paths := make([]git.File, 0, len(args))

	for _, p := range args {
		paths = append(paths, git.File(p))
	}

	matches, err := git.CheckIgnore(c, opts, paths)

	if err != nil {
		return err
	}

	exitCode := 1
	for _, match := range matches {
		if match.Pattern != "" {
			if !quiet && !verbose {
				fmt.Printf("%s\n", match.PathName.String())
			} else if !quiet && verbose {
				fmt.Printf("%s\n", match)
			}
			exitCode = 0
		}
	}

	os.Exit(exitCode)
	return nil
}
