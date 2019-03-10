package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func RevParse(c *git.Client, args []string) ([]git.ParsedRevision, git.RevParseOptions, error) {
	// We need to manually parse flags, because they're context sensitive.
	opts := git.RevParseOptions{}

	var parsedargs []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--quiet", "-q":
			opts.Quiet = true
		case "--verify":
			opts.Verify = true
		case "--default":
			switch i {
			case len(args) - 1:
				return nil, opts, fmt.Errorf("Must provide parameter for --default")
			default:
				opts.Default = args[i+1]
				i++
			}
		case "--help":
			flag.Usage()
			os.Exit(0)
		default:
			parsedargs = append(parsedargs, args[i])
		}
	}

	commits, err := git.RevParse(c, opts, parsedargs)
	return commits, opts, err
}
