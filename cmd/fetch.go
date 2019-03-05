package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

// These options can be shared with other subcommands that fetch, such as pull
func addSharedFetchFlags(flags *flag.FlagSet, options *git.FetchOptions) {
	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"all", "a", "append", "unshallow", "update-shallow", "dry-run", "k", "keep", "multiple", "p", "prune", "P", "prune-tags", "n", "no-tags", "t", "tags", "no-recurse-submodules", "u", "update-head-ok", "q", "quiet", "v", "verbose", "progress", "4", "ipv4", "ipv6"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"deepend", "shallow-since", "shallow-exclude", "refmap", "recurse-submodules", "j", "jobs", "submodule-prefix", "recurse-submodules-default", "upload-pack", "o", "server-option"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	options.Depth = int32(*flags.Int("depth", 0, "Limit fetching to the specified number of commits. This is current a no-op."))
}

func Fetch(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("fetch", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.FetchOptions{}
	addSharedFetchFlags(flags, &opts)
	flags.BoolVar(&opts.Force, "force", false, "Do not verify if refs exist before overwriting")
	flags.BoolVar(&opts.Force, "f", false, "Alias of --force")
	flags.Parse(args)

	var repository git.Remote
	var refspecs []git.RefSpec
	if flags.NArg() < 1 {
		repository = "origin" // FIXME origin is the default unless the current branch unless there is an upstream branch configured for the current branch
	} else if flags.NArg() >= 1 {
		repository = git.Remote(flags.Arg(0))
		for _, ref := range flags.Args()[1:] {
			refspecs = append(refspecs, git.RefSpec(ref))
		}
	}
	return git.Fetch(c, opts, repository, refspecs)
}
