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

// Matches determines whether a Ref matches a pattern according
// to the rules of ShowRef
func (r Ref) Matches(pattern string) bool {
	return r.Name == pattern || strings.HasSuffix(r.Name, "/"+pattern)
}

// MatchesRSSrc determines whether the ref matches the src of
// a refspec.
// If it matches the src, it also returns the destination
func (r Ref) MatchesRefSpecSrc(spec RefSpec) (match bool, dst Refname) {
	srcs := string(spec.Src())
	if srcs == r.Name {
		return true, spec.Dst()
	}
	if strings.HasSuffix(srcs, "/*") {
		prefix := strings.TrimSuffix(srcs, "/*")
		if strings.HasPrefix(r.Name, prefix) {
			dst := string(spec.Dst())
			if !strings.HasSuffix(dst, "/*") {
				// Src ended in glob but dst didn't
				panic("Invalid destination for refspec")
			}
			newprefix := strings.TrimSuffix(dst, "/*")
			base := strings.TrimPrefix(r.Name, prefix)
			return true, Refname(newprefix + base)
		}
	}
	return false, ""
}

func (r Ref) String() string {
	return fmt.Sprintf("%v %v", r.Value, r.Name)
}

func (r Ref) TabString() string {
	return fmt.Sprintf("%v\t%v", r.Value, r.Name)
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
	if opts.Verify {
		// If verify is specified, everything must be an exact match
		for _, ref := range patterns {
			if f := c.GitDir.File(File(ref)); !f.Exists() {
				return nil, fmt.Errorf("fatal: '%v' - not a valid ref", ref)
			}
			r, err := parseRef(c, ref)
			if err == InvalidCommit {
				return nil, fmt.Errorf("git show-ref: bad ref: %v (%v)", r.Name, r.Value)
			} else if err != nil {
				return nil, err
			}
			vals = append(vals, r)
			deref, err := getDeref(c, opts, r)
			if err != nil {
				return nil, err
			}
			if deref != nil {
				vals = append(vals, *deref)
			}
		}
		return vals, nil
	}
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
				ref, err := parseRef(c, refname)
				if err != nil && err != InvalidCommit {
					// Invalid commit can just mean we don't
					// have a local copy of the commit, so
					// we don't care for the purpose of show-ref
					return err
				}
				if len(patterns) == 0 {

					vals = append(vals, ref)
					deref, err := getDeref(c, opts, ref)
					if err != nil {
						return err
					}
					if deref != nil {
						vals = append(vals, *deref)
					}
					return nil
				}
				for _, p := range patterns {
					if ref.Matches(p) {
						vals = append(vals, ref)
						deref, err := getDeref(c, opts, ref)
						if err != nil {
							return err
						}
						if deref != nil {
							vals = append(vals, *deref)
						}
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
			ref, err := parseRef(c, refname)
			if err != nil {
				return nil, err
			}
			if len(patterns) == 0 {
				vals = append(vals, ref)
				deref, err := getDeref(c, opts, ref)
				if err != nil {
					return nil, err
				}
				if deref != nil {
					vals = append(vals, *deref)
				}
				continue
			}
			for _, p := range patterns {
				if ref.Matches(p) {
					vals = append(vals, ref)
					deref, err := getDeref(c, opts, ref)
					if err != nil {
						return nil, err
					}
					if deref != nil {
						vals = append(vals, *deref)
					}
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
			ref, err := parseRef(c, refname)
			if err != nil {
				return nil, err
			}
			if len(patterns) == 0 {
				vals = append(vals, ref)
				deref, err := getDeref(c, opts, ref)
				if err != nil {
					return nil, err
				}
				if deref != nil {
					vals = append(vals, *deref)
				}
				continue
			}
			for _, p := range patterns {
				if ref.Matches(p) {
					vals = append(vals, ref)
					deref, err := getDeref(c, opts, ref)
					if err != nil {
						return nil, err
					}
					if deref != nil {
						vals = append(vals, *deref)
					}
					break
				}
			}
		}
	}

	return vals, nil
}

func parseRef(c *Client, filename string) (Ref, error) {
	refname := strings.TrimPrefix(filename, "/")
	data, err := ioutil.ReadFile(c.GitDir.File(File(refname)).String())
	if err != nil {
		return Ref{}, err
	}
	if strings.HasPrefix(string(data), "ref: ") {
		deref, err := SymbolicRefGet(c, SymbolicRefOptions{}, SymbolicRef(refname))
		if err != nil {
			return Ref{}, err
		}
		sha1, err := deref.Sha1(c)
		if err != nil {
			return Ref{}, err
		}
		return Ref{refname, sha1}, nil
	} else {
		sha1, err := Sha1FromString(string(data))
		if err != nil {
			return Ref{}, err
		}
		// check for a dangling ref
		if _, err := c.GetObject(sha1); err != nil {
			return Ref{refname, sha1}, InvalidCommit
		}
		return Ref{refname, sha1}, nil
	}
}

func getDeref(c *Client, opts ShowRefOptions, ref Ref) (*Ref, error) {
	if !opts.Dereference {
		return nil, nil
	}
	if ref.Value.Type(c) == "tag" {
		deref, err := RevParse(c, RevParseOptions{}, []string{ref.Name + "^0"})
		if err != nil {
			return nil, err
		}
		return &Ref{ref.Name + "^{}", deref[0].Id}, nil
	}
	return nil, nil
}
