package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

// Sets up common options for git diff-files, diff-index and diff-tree and parses
// them.
//
// Any unique parameters must already be set up in flags before calling this,
// because this will call flags.Parse
func parseCommonDiffFlags(c *git.Client, options *git.DiffCommonOptions, defaultPatch bool, flags *flag.FlagSet, args []string) (newargs []string, err error) {
	patch := flags.Bool("patch", defaultPatch, "Generate patch")
	p := flags.Bool("p", defaultPatch, "Alias for --patch")
	u := flags.Bool("u", defaultPatch, "Alias for --patch")

	nopatch := flags.Bool("no-patch", false, "Suppress patch generation")
	s := flags.Bool("s", false, "Alias of --no-patch")
	unified := flags.Int("unified", 3, "Generate <n> lines of context")
	U := flags.Int("U", 3, "Alias of --unified")
	flags.BoolVar(&options.Raw, "raw", true, "Generate the diff in raw format")

	flags.Parse(args)
	args = flags.Args()

	if *patch || *p || *u {
		options.Patch = true
		options.Raw = false
	}
	if *nopatch || *s {
		options.Patch = false
	}

	if *unified != 3 && *U != 3 {
		fmt.Fprintf(flag.CommandLine.Output(), "Can not specify both --unified and -U\n")
		flags.Usage()
		os.Exit(2)
	} else if *unified != 3 {
		options.NumContextLines = *unified
	} else if *U != 3 {
		options.NumContextLines = *U
	} else {
		options.NumContextLines = 3
	}
	return args, nil
}

// Print the diffs that come back from either diff-files, diff-index, or diff-tree
// in the appropriate format according to options.
func printDiffs(c *git.Client, options git.DiffCommonOptions, diffs []git.HashDiff) error {
	return git.GeneratePatch(c, options, diffs, nil)
}
