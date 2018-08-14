package git

import (
	"fmt"
)

type MergeBaseOptions struct {
	Octopus bool
}

func MergeBase(c *Client, options MergeBaseOptions, commits []Commitish) (CommitID, error) {
	if len(commits) == 2 {
		// If there's only two commits specified, and one is
		// an ancestor of the other, than that one is the merge-base
		cmt0, err := commits[0].CommitID(c)
		if err != nil {
			return CommitID{}, err
		}
		cmt1, err := commits[1].CommitID(c)
		if err != nil {
			return CommitID{}, err
		}
		if cmt0.IsAncestor(c, cmt1) {
			return cmt0, nil
		} else if cmt1.IsAncestor(c, cmt0) {
			return cmt1, nil
		}
	}
	if options.Octopus {
		return MergeBaseOctopus(c, options, commits)
	}
	if len(commits) <= 1 {
		return CommitID{}, fmt.Errorf("MergeBase requires at least 2 commits")
	}

	// Starting with commits[0]'s parents, perform a bread-first search
	// looking for a commit who's an ancestor of commits[1:]
	tip := commits[0]
	cmt, err := tip.CommitID(c)
	if err != nil {
		return CommitID{}, err
	}

	// the first level is commit[0]'s parents.
	nextlevel, err := cmt.Parents(c)
	if err != nil {
		return CommitID{}, err
	}

	// Keep looking until there's no commits left.
	for len(nextlevel) > 0 {
		// Check if we've found a commit who's an ancestor
		// of another commit that was passed. If so, this is
		// the common ancestor of the "hypothetical merge commit"
		// that git-merge-base(1) talks about.
		for _, parent := range commits[1:] {
			cmt2, err := parent.CommitID(c)
			if err != nil {
				return CommitID{}, err
			}
			if cmt2.IsAncestor(c, cmt) {
				return cmt2, nil
			}
		}

		// Found nothing, so create a new queue of the
		// next level of parents to check.
		newnextlevel := make([]CommitID, 0)
		for _, parent := range nextlevel {
			parents, err := parent.Parents(c)
			if err != nil {
				return CommitID{}, err
			}
			newnextlevel = append(newnextlevel, parents...)
		}
		nextlevel = newnextlevel
	}

	// If nothing was found it's not an error, it just means the
	// merge-base is 00000000000000000000
	return CommitID{}, nil
}

func MergeBaseOctopus(c *Client, options MergeBaseOptions, commits []Commitish) (CommitID, error) {
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
