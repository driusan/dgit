package cmd

import (
	"fmt"

	"github.com/driusan/dgit/git"
)

func FetchPack(c *git.Client, args []string) error {
	flags := newFlagSet("fetch-pack")
	opts := git.FetchPackOptions{}
	flags.BoolVar(&opts.All, "all", false, "Fetch all remote refs")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Do not print indexing status")
	flags.BoolVar(&opts.Quiet, "q", false, "Alias of --quiet")
	flags.BoolVar(&opts.Keep, "keep", false, "Not implemented")
	flags.BoolVar(&opts.Keep, "k", false, "Not implemented")
	flags.BoolVar(&opts.Thin, "thin", false, "Fetches a thin pack (Not implemented)")
	flags.BoolVar(&opts.IncludeTag, "include-tag", false, "Send annotated tags along with other objects")
	flags.BoolVar(&opts.NoProgress, "no-progress", false, "Do not show progress information")
	flags.StringVar(&opts.UploadPack, "upload-pack", "", "Execute upload-pack instead of git-upload-pack")
	flags.StringVar(&opts.UploadPack, "exec", "", "Execute upload-pack instead of git-upload-pack")
	depth := flags.Int("depth", 2147483647, "Not implemented")
	_ = flags.String("shallow-since", "", "Not implemented")
	_ = flags.String("shallow-exclude", "", "Not implemented")
	flags.BoolVar(&opts.DeepenRelative, "deepen-relative", false, "Not implemented")
	flags.BoolVar(&opts.CheckSelfContainedAndConnected, "check-self-contained-and-connected", false, "Not implemented")
	flags.BoolVar(&opts.Verbose, "verbose", false, "Be more verbose")
	flags.Parse(args)
	args = flags.Args()
	opts.Depth = int(*depth)
	if len(args) < 1 {
		flags.Usage()
		return fmt.Errorf("Invalid flag usage")
	}
	rmt := git.Remote(args[0])
	var wants []git.Refname
	for _, name := range args[1:] {
		wants = append(wants, git.Refname(name))
	}
	_, err := git.FetchPack(c, opts, rmt, wants)
	return err
}
