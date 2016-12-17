package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	libgit "github.com/driusan/git"
	"github.com/driusan/go-git/zlib"
	//"os"
)

type Sha1 [20]byte
type CommitID Sha1
type TreeID Sha1
type BlobID Sha1

type Treeish interface {
	TreeID(c *Client) (TreeID, error)
}

type Commitish interface {
	CommitID(c *Client) (CommitID, error)
}

func Sha1FromString(s string) (Sha1, error) {
	b, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return Sha1{}, err
	}
	return Sha1FromSlice(b)
}

func Sha1FromSlice(s []byte) (Sha1, error) {
	if len(s) != 20 {
		return Sha1{}, fmt.Errorf("Invalid Sha1 %x (Size: %d)", s, len(s))
	}
	var val Sha1
	for i, b := range s {
		val[i] = b
	}
	return val, nil
}

func (s Sha1) String() string {
	return fmt.Sprintf("%x", string(s[:]))
}

func (s TreeID) String() string {
	return fmt.Sprintf("%x", string(s[:]))
}

func (s CommitID) String() string {
	return fmt.Sprintf("%x", string(s[:]))
}

func (s CommitID) CommitID(c *Client) (CommitID, error) {
	return s, nil
}

// Writes the object to w in compressed form
func (s Sha1) CompressedWriter(repo *libgit.Repository, w io.Writer) error {
	id, err := libgit.NewId(s[:])
	if err != nil {
		return err
	}

	_, _, uncompressed, err := repo.GetRawObject(id, false)
	zw := zlib.NewWriter(w)
	defer zw.Close()
	tee := io.TeeReader(uncompressed, zw)
	ioutil.ReadAll(tee)
	return nil
}

func (s Sha1) UncompressedSize(repo *libgit.Repository) uint64 {
	id, err := libgit.NewId(s[:])
	if err != nil {
		panic(err)
	}
	_, size, _, err := repo.GetRawObject(id, true)
	if err != nil {
		return 0
	}
	return uint64(size)
}

func (s Sha1) PackEntryType(repo *libgit.Repository) PackEntryType {
	id, err := libgit.NewId(s[:])
	if err != nil {
		panic(err)
	}
	t, _, _, _ := repo.GetRawObject(id, true)
	switch t {
	case libgit.ObjectCommit:
		return OBJ_COMMIT
	case libgit.ObjectTree:
		return OBJ_TREE
	case libgit.ObjectBlob:
		return OBJ_BLOB
	case libgit.ObjectTag:
		return OBJ_TAG
	default:
		panic("Unknown Object type")
	}
}

// Returns the git type of the object this Sha1 represents
func (s Sha1) Type(c *Client) string {
	// Temporary hack. Replace with a proper implementation.
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		panic(err)
	}
	switch s.PackEntryType(repo) {
	case OBJ_COMMIT:
		return "commit"
	case OBJ_TREE:
		return "tree"
	case OBJ_BLOB:
		return "blob"
	case OBJ_TAG:
		return "tag"
	default:
		return ""
	}
}

func (child CommitID) IsAncestor(c *Client, parent Commitish) bool {
	p, err := parent.CommitID(c)
	if err != nil {
		return false
	}
	ancestors := p.Ancestors(c)
	for _, c := range ancestors {
		if c == child {
			return true
		}
	}
	return false
}

func (s CommitID) Ancestors(c *Client) (commits []CommitID) {
	// TODO: Replace this with Parent(n) which returns the nth parent,
	// then improve the efficiency of everywhere that uses this. For now
	// we still depend on libgit. :(
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return nil
	}

	lgCommits, err := repo.CommitsBefore(s.String())
	if err != nil {
		return nil
	}
	for e := lgCommits.Front(); e != nil; e = e.Next() {
		c, ok := e.Value.(*libgit.Commit)
		if ok {
			sha, err := Sha1FromString(c.Id.String())
			if err != nil {
				continue
			}

			commits = append(commits, CommitID(sha))

		}
	}
	return
}

func NearestCommonParent(c *Client, com, other Commitish) (CommitID, error) {
	s, err := com.CommitID(c)
	if err != nil {
		return CommitID{}, err
	}
	ancestors := s.Ancestors(c)
	for _, commit := range ancestors {
		// This is a horrible algorithm. TODO: Do something better than O(n^3)
		if commit.IsAncestor(c, other) {
			return commit, nil
		}
	}
	// Nothing in common isn't an error, it just means the nearest common parent
	// is 0 (the empty commit)
	return CommitID{}, nil
}

func (c CommitID) GetAllObjects(cl *Client) ([]Sha1, error) {
	var objects []Sha1
	tree, err := c.TreeID(cl)
	if err != nil {
		return nil, err
	}
	objects = append(objects, Sha1(tree))
	children, err := tree.GetAllObjects(cl)
	if err != nil {
		return nil, err
	}
	objects = append(objects, children...)
	return objects, nil
}
func (t TreeID) GetAllObjects(cl *Client) ([]Sha1, error) {
	// TODO: Move this to client_hacks, or fix it to not depend on libgit
	repo, err := libgit.OpenRepository(cl.GitDir.String())
	if err != nil {
		return nil, err
	}

	var objects []Sha1
	tree, err := repo.GetTree(t.String())
	if err != nil {
		return nil, err
	}

	entries := tree.ListEntries()
	for _, o := range entries {
		switch o.Type {
		case libgit.ObjectBlob:
			sha, err := Sha1FromString(o.Id.String())
			if err != nil {
				panic(err)
			}
			objects = append(objects, sha)
		case libgit.ObjectTree:
			sha, err := Sha1FromString(o.Id.String())
			if err != nil {
				panic(err)
			}
			objects = append(objects, sha)
		}
	}
	return objects, nil
}

func (c CommitID) TreeID(cl *Client) (TreeID, error) {
	// TODO: Move this to client_hacks, or fix it to not depend on libgit
	repo, err := libgit.OpenRepository(cl.GitDir.String())
	if err != nil {
		return TreeID{}, err
	}

	commit, err := repo.GetCommit(fmt.Sprintf("%s", c))
	if err != nil {
		return TreeID{}, err
	}
	s, err := Sha1FromString(commit.Tree.String())
	return TreeID(s), err
}
