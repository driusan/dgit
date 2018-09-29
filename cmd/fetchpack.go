package cmd

import (
	"fmt"

	"github.com/driusan/dgit/git"
)

func FetchPack(c *git.Client, args []string) error {
	flags := newFlagSet("fetch-pack")
	opts := git.FetchPackOptions{}
	flags.Parse(args)
	args = flags.Args()
	if len(args) < 1 {
		flags.Usage()
		return fmt.Errorf("Invalid flag usage")
	}
	rmt := git.Remote(args[0])
	var wants []git.Refname
	for _, name := range args[1:] {
		wants = append(wants, git.Refname(name))
	}
	return git.FetchPack(c, opts, rmt, wants, nil)
}
