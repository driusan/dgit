package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Ref struct {
	// The full name of a ref, starting with refs/
	Name string

	// Value can be either a commit or a tag object
	Value Sha1
}

func (r Ref) Matches(pattern string) bool {
	return r.Name == pattern || strings.HasSuffix(r.Name, "/"+pattern)
}

func (r Ref) String() string {
	return fmt.Sprintf("%v %v", r.Value, r.Name)
}

func (r Ref) CommitID(c *Client) (CommitID, error) {
	switch r.Value.Type(c) {
	case "commit":
		return CommitID(r.Value), nil
	default:
		return CommitID{}, fmt.Errorf("Can not convert %v to commit", r.Name)
	}
}

func (r Ref) TreeID(c *Client) (TreeID, error) {
	switch r.Value.Type(c) {
	case "commit":
		return CommitID(r.Value).TreeID(c)
	case "tree":
		return TreeID(r.Value), nil
	default:
		return TreeID{}, fmt.Errorf("Can not get tree for %v", r.Name)
	}
}

type ShowRefOptions struct {
	IncludeHead bool

	Tags, Heads bool

	Dereference bool

	Sha1Only bool

	Verify bool

	Abbrev int

	Quiet bool

	ExcludeExisting string
}

func ShowRef(c *Client, opts ShowRefOptions, patterns []string) ([]Ref, error) {
	var vals []Ref
	if opts.IncludeHead {
		hcid, err := c.GetHeadCommit()
		if err == nil {
			// If the HEAD reference is a symbolic ref to something that
			// doesn't exist it's not an invalid state of git, we just
			// don't include it in the list.
			vals = append(vals, Ref{"HEAD", Sha1(hcid)})
		}
	}
	// FIXME: Include packed refs
	if !opts.Heads && !opts.Tags {
		err := filepath.Walk(c.GitDir.File("refs").String(),
			func(path string, info os.FileInfo, err error) error {
				if info.IsDir() {
					return nil
				}
				refname := strings.TrimPrefix(path, c.GitDir.String())
				refname = strings.TrimPrefix(refname, "/")

				data, err := ioutil.ReadFile(c.GitDir.File(File(refname)).String())
				if err != nil {
					return err
				}
				var sha1 Sha1
				if strings.HasPrefix(string(data), "ref: ") {
					deref, err := SymbolicRefGet(c, SymbolicRefOptions{}, SymbolicRef(refname))
					if err != nil {
						return err
					}
					sha1a, err := deref.Sha1(c)
					if err != nil {
						return err
					}
					sha1 = sha1a
				} else {
					sha1a, err := Sha1FromString(string(data))
					if err != nil {
						fmt.Println(refname)
						fmt.Println(string(data))
						return err
					}
					sha1 = sha1a
				}
				ref := Ref{refname, sha1}
				if len(patterns) == 0 {

					vals = append(vals, ref)
					return nil
				}
				for _, p := range patterns {
					if ref.Matches(p) {
						vals = append(vals, ref)
						return nil
					}
				}
				return nil
			},
		)
		if err != nil {
			return nil, err
		}
		return vals, nil
	}
	if opts.Heads {
		heads, err := ioutil.ReadDir(c.GitDir.File("refs/heads").String())
		if err != nil {
			return nil, err
		}
		for _, ref := range heads {
			refname := "refs/heads/" + ref.Name()
			data, err := ioutil.ReadFile(c.GitDir.File(File(refname)).String())
			if err != nil {
				return nil, err
			}
			sha1, err := Sha1FromString(string(data))
			if err != nil {
				return nil, err
			}
			ref := Ref{refname, sha1}
			if len(patterns) == 0 {
				vals = append(vals, ref)
				continue
			}
			for _, p := range patterns {
				if ref.Matches(p) {
					vals = append(vals, ref)
					break
				}
			}
		}
	}
	if opts.Tags {
		tags, err := ioutil.ReadDir(c.GitDir.File("refs/tags").String())
		if err != nil {
			return nil, err
		}
		for _, ref := range tags {
			refname := "refs/tags/" + ref.Name()
			data, err := ioutil.ReadFile(c.GitDir.File(File(refname)).String())
			if err != nil {
				return nil, err
			}
			sha1, err := Sha1FromString(string(data))
			if err != nil {
				return nil, err
			}
			ref := Ref{refname, sha1}
			if len(patterns) == 0 {
				vals = append(vals, ref)
				continue
			}
			for _, p := range patterns {
				if ref.Matches(p) {
					vals = append(vals, ref)
					break
				}
			}
		}
	}

	return vals, nil
}
