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
		if err != nil {
			return nil, err
		}
		vals = append(vals, Ref{"HEAD", Sha1(hcid)})
	}
	// FIXME: If nothing was included, walk everything (including remotes)
	// FIXME2: Include packed refs
	// FIXME3: Match patterns
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
				sha1, err := Sha1FromString(string(data))
				if err != nil {
					return err
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
