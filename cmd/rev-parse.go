package cmd

import (
	"flag"
	"os"

	"github.com/driusan/dgit/git"
)

func RevParse(c *git.Client, args []string) ([]git.ParsedRevision, git.RevParseOptions, error) {
	flags := newFlagSet("rev-parse")
	opts := git.RevParseOptions{}

	flags.BoolVar(&opts.Verify, "verify", false, "Verify a single object")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Be quiet")
	flags.BoolVar(&opts.Quiet, "q", false, "Alias of -quiet")
	flags.StringVar(&opts.Default, "default", "", "Specify a default value if no user input provided")
	flags.Parse(args)
	args = flags.Args()

	// --default can be specified at any arbitrary location, so we don't
	// depend on flags.Parse() to handle it.
	for i, arg := range args {
		if arg == "--default" && i != len(args)-1 {
			opts.Default = args[i+1]
			args = append(args[0:i], args[i+2:]...)
			break
		}
	}
	if len(args) == 1 && args[0] == "--help" {
		flag.Usage()
		os.Exit(0)
	}

	commits, err := git.RevParse(c, opts, args)
	return commits, opts, err
}
