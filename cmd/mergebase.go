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
	os.Args = append([]string{"git merge-base"}, args...)
	octopus := flag.Bool("octopus", false, "Compute the common ancestor of all supplied commits")
	ancestor := flag.Bool("is-ancestor", false, "Determine if two commits are ancestors")
	flag.Parse()
	args = flag.Args()

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
	} else if *octopus {
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

		return git.MergeBaseOctopus(c, asCommitish)
	} else {
		panic("Only octopus and is-ancestor are currently supported")
	}

	panic("Only octopus and is-ancestor are currently supported")
}
