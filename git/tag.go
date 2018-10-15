package git

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
)

// TagOptions is a stub for when more of Tag is implemented
type TagOptions struct {
	// Replace existing tags instead of erroring out.
	Force bool

	// Display tags
	List       bool
	IgnoreCase bool
}

// List tags, if tagnames is specified only list tags which match one
// of the patterns provided.
func TagList(c *Client, opts TagOptions, patterns []string) ([]string, error) {
	if len(patterns) != 0 {
		return nil, fmt.Errorf("Tag list with patterns not implemented")
	}

	files, err := ioutil.ReadDir(filepath.Join(c.GitDir.String(), "refs", "tags"))
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, f := range files {
		tags = append(tags, f.Name())
	}
	sort.Slice(tags, func(i, j int) bool {
		if opts.IgnoreCase {
			return strings.ToLower(tags[i]) < strings.ToLower(tags[j])
		}
		return tags[i] < tags[j]
	})
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
