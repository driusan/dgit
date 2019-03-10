package git

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TagOptions is a stub for when more of Tag is implemented
type TagOptions struct {
	// Replace existing tags instead of erroring out.
	Force bool

	// Display tags
	List       bool
	IgnoreCase bool

	Annotated bool

	// Delete the given tag
	Delete bool
}

// List tags, if tagnames is specified only list tags which match one
// of the patterns provided.
func TagList(c *Client, opts TagOptions, patterns []string) ([]string, error) {
	if len(patterns) != 0 {
		return nil, fmt.Errorf("Tag list with patterns not implemented")
	}

	files := []string{}

	err := filepath.Walk(filepath.Join(c.GitDir.String(), "refs", "tags"),
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				files = append(files, path)
			}
			return nil
		})
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, f := range files {
		tags = append(tags, f[len(c.GitDir.String())+len("/refs/tags/"):])
	}
	sort.Slice(tags, func(i, j int) bool {
		if opts.IgnoreCase {
			return strings.ToLower(tags[i]) < strings.ToLower(tags[j])
		}
		return tags[i] < tags[j]
	})
	return tags, nil
}

func TagCommit(c *Client, opts TagOptions, tagname string, cmt Commitish, msg string) error {
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
	if opts.Annotated {
		t := time.Now()
		tagger, err := c.GetCommitter(&t)
		tagstdin := fmt.Sprintf(`object %v
type commit
tag %v
tagger %v

%v`, comm, tagname, tagger, msg)
		tagid, err := Mktag(c, strings.NewReader(tagstdin))
		if err != nil {
			return err
		}
		// Pretend it's a CommitID for update-ref's sake.
		comm = CommitID(tagid)
	}
	return UpdateRefSpec(c, UpdateRefOptions{}, refspec, comm, "")
}

func TagDelete(c *Client, opts TagOptions, tagnames []Refname) error {
	if !opts.Delete {
		return fmt.Errorf("Must pass Delete option")
	}

	if len(tagnames) == 0 {
		return fmt.Errorf("No tags provided to delete")
	}
	// Convert the given tags to the full ref name
	var tagpatterns []string
	for _, tag := range tagnames {
		tagpatterns = append(tagpatterns, tag.String())
	}
	tags, err := ShowRef(c, ShowRefOptions{Tags: true}, tagpatterns)
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		return fmt.Errorf("No matching tags found")
	}
	for _, tag := range tags {
		if !strings.HasPrefix(tag.Name, "refs/tags") {
			return fmt.Errorf("Invalid tag: %v", tag.Name)
		}
		file := c.GitDir.File(File(tag.Name))
		if err := os.Remove(file.String()); err != nil {
			return err
		}
	}
	return nil
}
