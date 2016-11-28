package main

import (
	"fmt"
	"io"
	"io/ioutil"

	libgit "github.com/driusan/git"
	"github.com/driusan/go-git/zlib"
	//"os"
)

type Sha1 []byte

func (s Sha1) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%x", string(s))
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
