package cmd

import (
	"fmt"
        "os"

	"github.com/driusan/dgit/git"
)

func RevParse(c *git.Client, args []string) (commits []git.ParsedRevision, err2 error) {
	if len(args) == 1 && args[0] == "--help" {
		fmt.Printf("Usage: dgit parse-rev <commid>\n")
                os.Exit(0)
	}
	return git.RevParse(c, git.RevParseOptions{}, args)
}
