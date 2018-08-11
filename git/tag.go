package git

import (
	"fmt"
)

// TagOptions is a stub for when more of Tag is implemented
type TagOptions struct {
	// Replace existing tags instead of erroring out.
	Force bool
}

func TagCommit(c *Client, opts TagOptions, tagname string, cmt Commitish) error {
	refspec := RefSpec("refs/tags/" + tagname)
	var comm CommitID
	if cmt == nil {
		cmmt, err := c.GetHeadCommit()
		if err != nil {
			return err
		}
		comm = cmmt
	} else {
		cmmt, err := cmt.CommitID(c)
		if err != nil {
			return err
		}
		comm = cmmt
	}
	if refspec.File(c).Exists() && !opts.Force {
		return fmt.Errorf("tag '%v' already exists", tagname)
	}
	return UpdateRefSpec(c, UpdateRefOptions{}, refspec, comm, "")
}
