package cmd

import (
	"github.com/driusan/dgit/git"
)

func RevParse(c *git.Client, args []string) (commits []git.ParsedRevision, err2 error) {
	return git.RevParse(c, git.RevParseOptions{}, args)
}
