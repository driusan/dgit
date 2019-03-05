package git

import (
	"bytes"
	"fmt"
	"strings"
)

// A RefSpec refers to a reference contained under .git/refs
type RefSpec string

func (r RefSpec) String() string {
	if len(r) < 1 {
		return ""
	}

	// This will only trim a single nil byte, but if there's more
	// than that we're doing something really wrong.
	return strings.TrimSpace(strings.TrimSuffix(string(r), "\000"))
}

// Src represents the ref name of the ref at the remote location for a refspec.
// for instance, in the refspec "refs/heads/foo:refs/remotes/origin/foo",
// src refers to refs/heads/foo, the name of the remote reference, while dst
// refers to refs/remotes/origin/master, the location that we store our cache
// of what that remote reference is.
func (r RefSpec) Src() Refname {
	if len(r) < 1 {
		return ""
	}

	if r[0] == '+' {
		r = r[1:]
	}
	if pos := strings.Index(string(r), ":"); pos >= 0 {
		return Refname(r[:pos])
	}

	// There was no ":", so we just take the name.
	return Refname(r)
}

// Dst represents the local destination of a remote ref in a refspec.
// For instance, in the refspec "refs/heads/foo:refs/remotes/origin/foo",
// Dst refers to "refs/remotes/origin/foo". If the refspec does not have an
// explicit Dst specified, Dst returns an empty refname.
func (r RefSpec) Dst() Refname {
	if len(r) < 1 {
		return ""
	}

	if r[0] == '+' {
		r = r[1:]
	}
	if pos := strings.Index(string(r), ":"); pos >= 0 {
		return Refname(r[pos+1:])
	}
	return ""
}

func (r RefSpec) HasPrefix(s string) bool {
	return strings.HasPrefix(r.String(), s)
}

// Returns the file that holds r.
func (r RefSpec) File(c *Client) File {
	return c.GitDir.File(File(r.String()))
}

// Returns the value of RefSpec in Client's GitDir, or the empty string
// if it doesn't exist.
func (r RefSpec) Value(c *Client) (string, error) {
	f := r.File(c)
	val, err := f.ReadAll()
	return strings.TrimSpace(val), err
}

func (r RefSpec) Sha1(c *Client) (Sha1, error) {
	v, err := r.Value(c)
	if err != nil {
		return Sha1{}, err
	}
	return Sha1FromString(v)
}

func (r RefSpec) CommitID(c *Client) (CommitID, error) {
	sha1, err := r.Sha1(c)
	if err != nil {
		return CommitID{}, err
	}
	switch t := sha1.Type(c); t {
	case "commit":
		return CommitID(sha1), nil
	case "tag":
		obj, err := c.GetObject(sha1)
		if err != nil {
			return CommitID{}, err
		}
		content := obj.GetContent()
		reader := bytes.NewBuffer(content)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return CommitID{}, err
		}
		objid := strings.TrimPrefix(string(line), "object ")
		objsha, err := CommitIDFromString(objid)
		if err != nil {
			return CommitID{}, err
		}
		line, err = reader.ReadBytes('\n')
		if err != nil {
			return CommitID{}, err
		}
		if string(line) != "type commit\n" {
			return CommitID{}, fmt.Errorf("Tag does not point to a commit: %v", string(line))
		}
		return objsha, nil
	default:
		return CommitID{}, fmt.Errorf("Invalid commit type %v", t)
	}
}

func (r RefSpec) TreeID(c *Client) (TreeID, error) {
	cmt, err := r.CommitID(c)
	if err != nil {
		return TreeID{}, err
	}
	return cmt.TreeID(c)
}

// A Branch is a type of RefSpec that lives under refs/heads/ or refs/remotes/heads
// Use GetBranch to get a valid branch from a branchname, don't cast from string
type Branch RefSpec

// Implements Stringer on Branch
func (b Branch) String() string {
	return RefSpec(b).String()
}

// Returns a valid Branch object for an existing branch.
func GetBranch(c *Client, branchname string) (Branch, error) {
	if b := Branch("refs/heads/" + branchname); b.Exists(c) {
		return b, nil
	}

	// remote branches are branches too!
	if b := Branch("refs/remotes/" + branchname); b.Exists(c) {
		return b, nil
	}
	return "", InvalidBranch
}

// Returns true if the branch exists under c's GitDir
func (b Branch) Exists(c *Client) bool {
	return c.GitDir.File(File(b)).Exists()
}

// Implements Commitish interface on Branch.
func (b Branch) CommitID(c *Client) (CommitID, error) {
	val, err := RefSpec(b).Value(c)
	if err != nil {
		return CommitID{}, err
	}
	sha, err := Sha1FromString(val)
	return CommitID(sha), err
}

// Implements Treeish on Branch.
func (b Branch) TreeID(c *Client) (TreeID, error) {
	cmt, err := b.CommitID(c)
	if err != nil {
		return TreeID{}, err
	}
	return cmt.TreeID(c)
}

// Returns the branch name, without the refspec portion.
func (b Branch) BranchName() string {
	s := string(b)
	if strings.HasPrefix(s, "refs/heads/") {
		return strings.TrimPrefix(s, "refs/heads/")
	}
	return strings.TrimPrefix(s, "refs/")
}

// Delete a branch
func (b Branch) DeleteBranch(c *Client) error {
	location := c.GitDir.File(File(b))
	err := location.Remove()
	if err != nil {
		return err
	}
	return nil
}
