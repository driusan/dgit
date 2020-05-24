package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

func Push(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("branch", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"all", "mirror", "tags", "follow-tags", "atomic", "n", "dry-run", "f", "force", "delete", "prune", "v", "verbose", "u", "no-signed", "no-verify"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"receive-pack", "repo", "o", "push-option", "signed", "force-with-lease"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	opts := git.PushOptions{}
	flags.BoolVar(&opts.SetUpstream, "set-upstream", false, "Sets the upstream remote for the branch")

	flags.Parse(args)

	args = flags.Args()
	var remote git.Remote
	var refnames []git.Refname
	branch := c.GetHeadBranch()
	if len(args) == 0 {

		remoteconfig := c.GetConfig("branch." + branch.BranchName() + ".remote")
		if remoteconfig != "" {
			remote = git.Remote(remoteconfig)
		} else {
			remote = git.Remote("origin")
		}
	} else {
		remote = git.Remote(args[0])
	}
	if len(args) < 2 {
		refnames = append(refnames, git.Refname(branch.BranchName()))
	} else {
		for _, ref := range args[1:] {
			refnames = append(refnames, git.Refname(ref))
		}

	}

	return git.Push(c, opts, remote, refnames)
}
