package cmd

import (
	"fmt"

	"github.com/driusan/dgit/git"
)

func FetchPack(c *git.Client, args []string) error {
	flags := newFlagSet("fetch-pack")
	opts := git.FetchPackOptions{}
	flags.StringVar(&opts.UploadPack, "upload-pack", "", "Execute upload-pack instead of git-upload-pack")
	flags.StringVar(&opts.UploadPack, "exec", "", "Execute upload-pack instead of git-upload-pack")
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
