package git

import (
	"fmt"
	"io/ioutil"
)

// TagOptions is a stub for when more of Tag is implemented
type TagOptions struct {
	// Replace existing tags instead of erroring out.
	Force bool

	List bool
}

// List tags, if tagnames is specified only list tags which match one
// of the patterns provided.
func TagList(c *Client, patterns []string) ([]string, error) {
	if len(patterns) != 0 {
		return nil, fmt.Errorf("Tag list with patterns not implemented")
	}

	files, err := ioutil.ReadDir(c.GitDir.String() + "/refs/tags")
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, f := range files {
		tags = append(tags, f.Name())
	}
	return tags, nil
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
