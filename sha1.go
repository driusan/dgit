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

type Sha1 []byte
type CommitID Sha1
type TreeID Sha1
type BlobID Sha1

func Sha1FromString(s string) (Sha1, error) {
	b, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}
	return Sha1(b), nil
}

func (s Sha1) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%x", string(s))
}

func (s TreeID) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%x", string(s))
}

// Converts from a slice to an array, mostly for working with indexes.
func (s Sha1) AsByteArray() (val [20]byte) {
	if len(s) != 20 {
		panic(fmt.Sprintf("Invalid Sha1 %x (Size: %d)", s, len(s)))
	}
	for i, b := range s {
		val[i] = b
	}
	return
}

// Writes the object to w in compressed form
func (s Sha1) CompressedWriter(repo *libgit.Repository, w io.Writer) error {
	id, err := libgit.NewId(s)
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
	id, err := libgit.NewId(s)
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
	id, err := libgit.NewId(s)
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

func (s Sha1) Ancestors(repo *libgit.Repository) (commits []CommitID) {
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

func (c CommitID) GetAllObjects(repo *libgit.Repository) ([]Sha1, error) {
	var objects []Sha1
	tree, err := c.GetTree(repo)
	if err != nil {
		return nil, err
	}
	objects = append(objects, Sha1(tree))
	children, err := tree.GetAllObjects(repo)
	objects = append(objects, children...)
	return objects, nil
}
func (t TreeID) GetAllObjects(repo *libgit.Repository) ([]Sha1, error) {
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
func (c CommitID) GetTree(repo *libgit.Repository) (TreeID, error) {
	commit, err := repo.GetCommit(fmt.Sprintf("%x", c))
	if err != nil {
		return nil, err
	}
	tid, err := Sha1FromString(commit.Tree.String())
	return TreeID(tid), err
}
