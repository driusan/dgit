package cmd

import (
	"flag"
	"os"

	"github.com/driusan/dgit/git"
)

func RevParse(c *git.Client, args []string) (commits []git.ParsedRevision, err2 error) {
	if len(args) == 1 && args[0] == "--help" {
		flag.Usage()
		os.Exit(0)
	}
	return git.RevParse(c, git.RevParseOptions{}, args)
}
