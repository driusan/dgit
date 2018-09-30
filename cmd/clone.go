package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Clone(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("clone", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.CloneOptions{}
	initOpts := git.InitOptions{}
	flags.BoolVar(&initOpts.Quiet, "quiet", false, "Operate quietly")
	flags.BoolVar(&initOpts.Quiet, "q", false, "Alias for --quiet")
	flags.BoolVar(&initOpts.Bare, "bare", false, "Make a bare Git repository.")
	template := ""
	flags.StringVar(&template, "template", "", "Specify the directory from which templates will be used.")

	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"l", "s", "no-hardlinks", "n", "mirror", "dissociate", "single-branch", "no-single-branch", "no-tags", "shallow-submodules", "no-shallow-submodules"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"o", "b", "u", "reference", "separate-git-dir", "depth", "recurse-submodules", "jobs"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	flags.Parse(args)

	if template != "" {
		initOpts.Template = git.File(template)
	}

	opts.InitOptions = initOpts
	var repoid git.Remote
	var dirName git.File
	// TODO: This argument parsing should be smarter and more
	// in line with how cgit does it.
	switch flags.NArg() {
	case 0:
		flags.Usage()
		os.Exit(2)
	case 1:
		repoid = git.Remote(flags.Arg(0))
	case 2:
		repoid = git.Remote(flags.Arg(0))
		dirName = git.File(flags.Arg(1))
	default:
		repoid = git.Remote(flags.Arg(0))
		dirName = git.File(flags.Arg(1))
	}

	return git.Clone(opts, repoid, dirName)
}
