package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

var Ancestor error = errors.New("Commit is an ancestor")
var NonAncestor error = errors.New("Commit not an ancestor")

func MergeBaseOctopus(c *Client, commits []Commitish) (CommitID, error) {
	var bestSoFar Commitish = commits[0]
	for _, commit := range commits[1:] {
		closest, err := NearestCommonParent(c, bestSoFar, commit)
		if err != nil {
			return CommitID{}, err
		}
		bestSoFar = closest
	}
	return bestSoFar.CommitID(c)

}
func MergeBase(c *Client, args []string) (CommitID, error) {
	os.Args = append([]string{"git merge-base"}, args...)
	octopus := flag.Bool("octopus", false, "Compute the common ancestor of all supplied commits")
	ancestor := flag.Bool("is-ancestor", false, "Determine if two commits are ancestors")
	flag.Parse()
	args = flag.Args()

	if len(args) < 1 {
		flag.Usage()
		return CommitID{}, fmt.Errorf("Invalid usage of merge-base")
	}
	if *ancestor {
		commits, err := RevParse(c, args)
		if err != nil {
			return CommitID{}, err
		}
		if commits[1].IsAncestor(c, commits[0]) {
			return CommitID{}, Ancestor
		}
		return CommitID{}, NonAncestor
	} else if *octopus {
		commits, err := RevParse(c, args)
		if err != nil {
			return CommitID{}, err
		}

		// RevParse returns ParsedRevisions, which are Commitish, but slices
		// can't be passed in terms of interfaces without converting them
		// first.
		var asCommitish []Commitish
		for _, c := range commits {
			asCommitish = append(asCommitish, c)
		}

		return MergeBaseOctopus(c, asCommitish)
	} else {
		panic("Only octopus and is-ancestor are currently supported")
	}

	panic("Only octopus and is-ancestor are currently supported")
}
