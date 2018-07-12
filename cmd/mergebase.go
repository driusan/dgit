package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

var Ancestor error = errors.New("Commit is an ancestor")
var NonAncestor error = errors.New("Commit not an ancestor")

func MergeBase(c *git.Client, args []string) (git.CommitID, error) {
	flags := flag.NewFlagSet("merge-base", flag.ExitOnError)
	flags.SetOutput(os.Stdout)
	var options git.MergeBaseOptions

	flags.BoolVar(&options.Octopus, "octopus", false, "Compute the common ancestor of all supplied commits")
	ancestor := flags.Bool("is-ancestor", false, "Determine if two commits are ancestors")
	flags.Parse(args)
	args = flags.Args()

	if len(args) < 1 {
		flag.Usage()
		return git.CommitID{}, fmt.Errorf("Invalid usage of merge-base")
	}
	if *ancestor {
		commits, err := RevParse(c, args)
		if err != nil {
			return git.CommitID{}, err
		}
		if commits[1].IsAncestor(c, commits[0]) {
			return git.CommitID{}, Ancestor
		}
		return git.CommitID{}, NonAncestor
	}

	commits, err := RevParse(c, args)
	if err != nil {
		return git.CommitID{}, err
	}

	// RevParse returns ParsedRevisions, which are Commitish, but slices
	// can't be passed in terms of interfaces without converting them
	// first.
	var asCommitish []git.Commitish
	for _, c := range commits {
		asCommitish = append(asCommitish, c)
	}

	return git.MergeBase(c, options, asCommitish)
}
