package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Pull(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("pull", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.PullOptions{}
	addSharedFetchFlags(flags, &opts.FetchOptions)
	addSharedMergeFlags(flags, &opts.MergeOptions)
	flags.Parse(args)

	var repository string
	var remotebranches []string
	config, err := git.LoadLocalConfig(c)
	if err != nil {
		return err
	}

	if flags.NArg() < 1 {
		// TODO simplistic, probably won't work in all cases
                // Instead the branch information should be retrieved from the merge config
		headbranch := c.GetHeadBranch().BranchName()
		repository, _ = config.GetConfig("branch." + headbranch + ".remote")
		//mergeremote, _ := config.GetConfig("branch." + head + ".merge")
		remotebranches = []string{fmt.Sprintf("%s/%s", repository, headbranch)}
	} else if flags.NArg() == 1 {
		repository = flags.Arg(0)
                // Instead the branch information should be retrieved from the merge config
		headbranch := c.GetHeadBranch().BranchName()
		//mergeremote, _ := config.GetConfig("branch." + headbranch + ".merge")
		remotebranches = []string{fmt.Sprintf("%s/%s", repository, headbranch)}
	} else if flags.NArg() >= 2 {
		repository = flag.Arg(0)
		remotebranches = flag.Args()[1:]
	} else {
		flags.Usage()
		os.Exit(1)
	}

	return git.Pull(c, opts, repository, remotebranches)
}
