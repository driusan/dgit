package git

import (
	"fmt"
	"io"
)

type BranchOptions struct {
	All   bool
	Quiet bool
}

func BranchList(c *Client, stdout io.Writer, opts BranchOptions, patterns []string) ([]Branch, error) {
	branches, err := c.GetBranches()
	if err != nil {
		return nil, err
	}
	if opts.All {
		rb, err := c.GetRemoteBranches()
		if err != nil {
			return nil, err
		}
		branches = append(branches, rb...)
	}
	if !opts.Quiet {
		head := c.GetHeadBranch()
		for _, b := range branches {
			if head == b {
				fmt.Println(" *", b.BranchName())
			} else {
				fmt.Println("  ", b.BranchName())
			}

		}
	}
	return branches, nil
}
