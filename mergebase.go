package main

import (
	"errors"
	"flag"
	"fmt"
	libgit "github.com/driusan/git"
	"os"
)

var Ancestor error = errors.New("Commit is an ancestor")
var NonAncestor error = errors.New("Commit not an ancestor")

func MergeBase(c *Client, repo *libgit.Repository, args []string) (CommitID, error) {
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
		if commits[1].Id.IsAncestor(repo, commits[0].Id) {
			return CommitID{}, Ancestor
		}
		return CommitID{}, NonAncestor
	} else if *octopus {
		commits, err := RevParse(c, args)
		if err != nil {
			return CommitID{}, err
		}
		bestSoFar := commits[0].Id
		for _, commit := range commits[1:] {
			closest, err := bestSoFar.NearestCommonParent(repo, commit.Id)
			if err != nil {
				return CommitID{}, err
			}
			bestSoFar = closest
		}
		return bestSoFar, nil
	} else {
		panic("Only octopus and is-ancestor are currently supported")
	}

	panic("Only octopus and is-ancestor are currently supported")
}
