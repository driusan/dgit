package git

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/driusan/dgit/zlib"
	libgit "github.com/driusan/git"
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

func CommitIDFromString(s string) (CommitID, error) {
	s1, err := Sha1FromString(s)
	return CommitID(s1), err
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
	return Sha1(s).String()
}

func (s CommitID) String() string {
	return Sha1(s).String()
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
	children, err := tree.GetAllObjects(cl, "", true, false)
	if err != nil {
		return nil, err
	}
	for _, s := range children {
		objects = append(objects, s.Sha1)
	}
	return objects, nil
}

// A TreeEntry represents an entry inside of a Treeish.
type TreeEntry struct {
	Sha1     Sha1
	FileMode EntryMode
}

// Returns a map of all paths in the Tree. If recurse is true, it will recurse
// into subtrees. If excludeself is true, it will *not* include it's own Sha1.
// (Only really meaningful with recurse)
func (t TreeID) GetAllObjects(cl *Client, prefix IndexPath, recurse, excludeself bool) (map[IndexPath]TreeEntry, error) {
	o, err := cl.GetObject(Sha1(t))
	if err != nil {
		return nil, err
	}

	if o.GetType() != "tree" {
		return nil, fmt.Errorf("%s is not a tree object", t)
	}

	treecontent := o.GetContent()
	entryStart := 0
	var sha Sha1
	val := make(map[IndexPath]TreeEntry)

	for i := 0; i < len(treecontent); i++ {
		// The format of each tree entry is:
		// 	[permission] [name] \0 [20 bytes of Sha1]
		// so if we find a \0, it means the next 20 bytes are the sha,
		// and we need to keep track of the previous entry start to figure
		// out the name and perm mode.
		if treecontent[i] == 0 {
			// Add 1 when converting the value of the sha1 because
			// because i is currently set to the nil
			sha, err = Sha1FromSlice(treecontent[i+1 : i+21])
			if err != nil {
				return nil, err
			}

			// Split up the perm from the name based on the whitespace
			split := bytes.SplitN(treecontent[entryStart:i], []byte{' '}, 2)
			perm := split[0]
			name := split[1]

			var mode EntryMode
			switch string(perm) {
			case "40000":
				mode = ModeTree
				if recurse {
					childTree := TreeID(sha)
					children, err := childTree.GetAllObjects(cl, "", recurse, excludeself)
					if err != nil {
						return nil, err
					}
					for child, childval := range children {
						val[IndexPath(name)+"/"+child] = childval

					}
				}
			case "100644":
				mode = ModeBlob
			case "100755":
				mode = ModeExec
			default:
				panic(fmt.Sprintf("Unsupported mode %v in tree %s", string(perm), t))
			}
			val[IndexPath(name)] = TreeEntry{
				Sha1:     sha,
				FileMode: mode,
			}
			i += 20
			entryStart = i + 1
		}

	}
	return val, nil
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

// Ensures the Tree implements Treeish
func (t TreeID) TreeID(cl *Client) (TreeID, error) {
	// Validate that it's a tree
	if Sha1(t).Type(cl) != "tree" {
		return TreeID{}, fmt.Errorf("Invalid tree")
	}
	return t, nil
}

// Converts the Tree into an IndexEntries, to simplify comparisons between
// Trees and Indexes
func GetIndexMap(c *Client, t Treeish) (map[IndexPath]*IndexEntry, error) {
	treeid, err := t.TreeID(c)
	if err != nil {
		return nil, err
	}
	indexentries, err := ExpandTreeIntoIndexesById(c, treeid.String(), true, false)
	if err != nil {
		return nil, err
	}
	// Create a fake index, to use the GetMap() function
	idx := &Index{Objects: indexentries}
	return idx.GetMap(), nil
}
