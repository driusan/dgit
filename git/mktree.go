package git

import (
	"io"
	//"reflect"
	"bytes"
	"fmt"
	"sort"
)

type MkTreeOptions struct {
	NilTerminate bool
	AllowMissing bool
	Batch        bool
}

// Mktree reads a tree from r in the same format as ls-tree and returns
// the TreeID of the converted tree.
func MkTree(c *Client, opts MkTreeOptions, r io.Reader) (TreeID, error) {
	var entry struct {
		mode, typ string
		sha1      string
		name      string
	}
	entries := []*IndexEntry{}
readlines:
	for {
		var err error
		if opts.NilTerminate {
			_, err = fmt.Fscanf(r, "%s %s %s\t %s\x00", &entry.mode, &entry.typ, &entry.sha1, &entry.name)
		} else {
			_, err = fmt.Fscanf(r, "%s %s %s\t %s\n", &entry.mode, &entry.typ, &entry.sha1, &entry.name)
		}
		switch err {
		case io.EOF:
			break readlines
		case nil:
		default:
			return TreeID{}, err
		}
		indexentry := &IndexEntry{}
		switch entry.mode {
		// FIXME: Validate typ too
		case "100644":
			indexentry.Mode = ModeBlob
		case "100755":
			indexentry.Mode = ModeExec
		case "120000":
			indexentry.Mode = ModeSymlink
		case "160000":
			indexentry.Mode = ModeCommit
		case "040000":
			indexentry.Mode = ModeTree
		default:
			return TreeID{}, fmt.Errorf("Invalid mode")
		}
		sha1, err := Sha1FromString(entry.sha1)
		if err != nil {
			return TreeID{}, err
		}
		indexentry.Sha1 = sha1
		if !opts.AllowMissing {
			have, _, _ := c.HaveObject(sha1)
			if !have {
				return TreeID{}, fmt.Errorf("Missing object %v", sha1)
			}
		}
		indexentry.PathName = IndexPath(entry.name)
		entries = append(entries, indexentry)
	}
	// format of each line: Mode filename\x00sha1
	sort.Sort(ByPath(entries))
	content := bytes.NewBuffer(nil)
	for _, entry := range entries {
		fmt.Fprintf(content, "%o %s\x00", entry.Mode, entry.PathName)
		if _, err := content.Write(entry.Sha1[:]); err != nil {
			return TreeID{}, err
		}
	}
	sha, err := c.WriteObject("tree", content.Bytes())
	return TreeID(sha), err
}

// MkTreeBatch reads trees separated by newlines from r, and writes the
// TreeID to w.
func MkTreeBatch(c *Client, opts MkTreeOptions, r io.Reader, w io.Writer) error {
	if !opts.Batch {
		return fmt.Errorf("MkTreeBatch requires batch option")
	}

	return fmt.Errorf("Not implemented")
}
