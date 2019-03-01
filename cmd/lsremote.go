package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func LsRemote(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("ls-remote", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.LsRemoteOptions{}
	flags.BoolVar(&opts.Heads, "heads", false, "Show only heads")
	flags.BoolVar(&opts.Heads, "h", false, "Alias of -heads")
	flags.BoolVar(&opts.Tags, "tags", false, "Show only tags")
	flags.BoolVar(&opts.Tags, "t", false, "Alias of -tags")
	flags.BoolVar(&opts.RefsOnly, "refs", false, "Do not show pseudo-refs or peeled tags in output")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Do not print matching refs")
	flags.BoolVar(&opts.Quiet, "q", false, "alias of --q")
	flags.StringVar(&opts.UploadPack, "upload-pack", "", "Specify the full path of git-upload-pack on the remote")
	flags.BoolVar(&opts.ExitCode, "exit-code", false, "Exit with status 2 when no matching refs are found")
	flags.BoolVar(&opts.GetURL, "get-url", false, "Expand the url without talking to the remote")
	flags.BoolVar(&opts.SymRef, "symref", false, "Show the underlying ref when showing a symbolic ref")
	flags.StringVar(&opts.Sort, "sort", "", "Sort based on the given key pattern")

	flags.Var(newMultiStringValue(&opts.ServerOptions), "server-option", "Transmit the option when using protocol versoin 2.")
	flags.Var(newMultiStringValue(&opts.ServerOptions), "o", "Alias of --server-option")

	flags.Parse(args)
	args = flags.Args()
	var repo git.Remote
	var patterns []string
	switch len(args) {
	case 0:
		// The LsRemote func in the git package sets it to origin internally
		repo = ""
	case 1:
		repo = git.Remote(args[0])
	default:
		repo = git.Remote(args[0])
		for _, ref := range args[1:] {
			patterns = append(patterns, ref)
		}
	}
	refs, err := git.LsRemote(c, opts, repo, patterns)
	if err != nil {
		return err
	}
	if opts.ExitCode && len(refs) == 0 {
		os.Exit(2)
	}
	for _, ref := range refs {
		fmt.Println(ref.TabString())
	}
	return nil
}
