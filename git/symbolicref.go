package git

import (
	"errors"
	"fmt"
	"strings"
)

var DetachedHead error = errors.New("In Detached HEAD state")

// A SymbolicRef is generally "HEAD". It's a fake symlink used by git
// to support operating systems that don't have symlinks
type SymbolicRef string

func (s SymbolicRef) String() string {
	return string(s)
}

func (s SymbolicRef) CommitID(c *Client) (CommitID, error) {
	rspec, err := SymbolicRefGet(c, SymbolicRefOptions{}, s)
	if err != nil {
		return CommitID{}, err
	}
	return rspec.CommitID(c)
}

// SymbolicRefOptions represents the command line options
// that may be passed on the command line. (NB. None of these
// are implemented.)
type SymbolicRefOptions struct {
	Quiet  bool
	Delete bool
	Short  bool
}

// Gets a RefSpec for a symbolic ref. Returns "" if symname is not a valid
// symbolic ref.
func SymbolicRefGet(c *Client, opts SymbolicRefOptions, symname SymbolicRef) (RefSpec, error) {
	file := c.GitDir.File(File(symname))

	value, err := file.ReadAll()
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(value, "ref: ") {
		return RefSpec(value), DetachedHead
	}
	if opts.Short {
		return RefSpec(strings.TrimPrefix(value, "ref: refs/heads/")), nil
	}
	return RefSpec(strings.TrimPrefix(value, "ref: ")), nil

}
func SymbolicRefDelete(c *Client, opts SymbolicRefOptions, symname SymbolicRef) error {
	file := c.GitDir.File(File(symname))
	if !file.Exists() {
		return fmt.Errorf("SymbolicRef %s does not exist.", symname)
	}
	return file.Remove()

}

func SymbolicRefUpdate(c *Client, opts SymbolicRefOptions, symname SymbolicRef, refvalue RefSpec, reason string) error {
	if !strings.HasPrefix(refvalue.String(), "refs/") {
		return fmt.Errorf("Refusing to point %s outside of refs/", symname)
	}
	file, err := c.GitDir.Create(File(symname))
	if err != nil {
		return err
	}
	defer file.Close()
	if reason != "" {
		reflog := c.GitDir.File(File("logs/" + symname.String()))
		if reflog.Exists() {
			updateReflog(c, false, reflog, symname, refvalue, reason)
		}
	}

	fmt.Fprintf(file, "ref: %s", refvalue)
	return nil
}
