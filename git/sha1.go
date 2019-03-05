package git

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/driusan/dgit/zlib"
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
func (s Sha1) CompressedWriter(c *Client, w io.Writer) error {
	obj, err := c.GetObject(s)
	if err != nil {
		return err
	}
	zw := zlib.NewWriter(w)
	defer zw.Close()
	tee := io.TeeReader(bytes.NewReader(obj.GetContent()), zw)
	ioutil.ReadAll(tee)
	return nil
}

func (s Sha1) UncompressedSize(c *Client) uint64 {
	_, sz, err := c.GetObjectMetadata(s)
	if err != nil {
		return 0
	}
	return sz
}

func (id Sha1) PackEntryType(c *Client) PackEntryType {
	switch id.Type(c) {
	case "commit":
		return OBJ_COMMIT
	case "tree":
		return OBJ_TREE
	case "blob":
		return OBJ_BLOB
	case "tag":
		return OBJ_TAG
	default:
		panic("Unknown Object type")
	}
}

// Returns the git type of the object this Sha1 represents
func (id Sha1) Type(c *Client) string {
	t, _, err := c.GetObjectMetadata(id)
	if err != nil {
		panic(err)
		return ""
	}
	return t
}

// Returns all direct parents of commit c.
func (cmt CommitID) Parents(c *Client) ([]CommitID, error) {
	obj, err := c.GetObject(Sha1(cmt))
	if err != nil {
		return nil, err
	}
	val := obj.GetContent()
	reader := bytes.NewBuffer(val)
	var parents []CommitID
	for line, err := reader.ReadBytes('\n'); err == nil; line, err = reader.ReadBytes('\n') {
		line = bytes.TrimSuffix(line, []byte{'\n'})
		if len(line) == 0 {
			return parents, nil
		}
		if bytes.HasPrefix(line, []byte("parent ")) {
			pc := string(bytes.TrimPrefix(line, []byte("parent ")))
			parent, err := CommitIDFromString(pc)
			if err != nil {
				return nil, err
			}
			parents = append(parents, parent)
		}
	}
	return parents, nil
}

func (child CommitID) IsAncestor(c *Client, parent Commitish) bool {
	p, err := parent.CommitID(c)
	if err != nil {
		return false
	}

	ancestorMap, err := p.AncestorMap(c)
	if err != nil {
		return false
	}

	_, ok := ancestorMap[child]
	return ok
}

var ancestorMapCache map[CommitID]map[CommitID]struct{}

// AncestorMap returns a map of empty structs (which can be interpreted as a set)
// of ancestors of a CommitID.
//
// It's useful if you want to know all the ancestors of s, but don't particularly
// care about their order. Since commits parents can never be changed, multiple
// calls to AncestorMap are cached and the cost of calculating the ancestory tree
// is only incurred the first time.
func (s CommitID) AncestorMap(c *Client) (map[CommitID]struct{}, error) {
	if cached, ok := ancestorMapCache[s]; ok {
		return cached, nil
	}
	m := make(map[CommitID]struct{})
	parents, err := s.Parents(c)
	if err != nil {
		return nil, err
	}
	var empty struct{}

	m[s] = empty

	for _, val := range parents {
		m[val] = empty
	}

	for _, p := range parents {
		grandparents, err := p.AncestorMap(c)
		if err != nil {
			return nil, err
		}
		for k, _ := range grandparents {
			m[k] = empty
		}
	}

	if ancestorMapCache == nil {
		ancestorMapCache = make(map[CommitID]map[CommitID]struct{})
	}
	ancestorMapCache[s] = m

	return m, nil

}

func (cmt CommitID) GetCommitterDate(c *Client) (time.Time, error) {
	obj, err := c.GetCommitObject(cmt)
	if err != nil {
		return time.Time{}, err
	}
	committerStr := obj.GetHeader("committer")

	if committerStr == "" {
		return time.Time{}, fmt.Errorf("Commit %s does not have a committer", cmt)
	}

	committerPieces := strings.Split(committerStr, " ")
	if len(committerPieces) < 3 {
		return time.Time{}, fmt.Errorf("Could not parse committer %s", committerStr)

	}

	unixTime, err := strconv.ParseInt(committerPieces[len(committerPieces)-2], 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	t := time.Unix(unixTime, 0)
	timeZone := committerPieces[len(committerPieces)-1]
	if loc, ok := tzCache[timeZone]; ok {
		return t.In(loc), nil
	}
	tzHours, err := strconv.ParseInt(timeZone[:len(timeZone)-2], 10, 64)
	if err != nil {
		panic(err)
	}
	loc := time.FixedZone(timeZone, int(tzHours*60*60))
	if tzCache == nil {
		tzCache = make(map[string]*time.Location)

	}
	tzCache[timeZone] = loc
	date := t.In(loc)

	return date, nil
}

var tzCache map[string]*time.Location

func (cmt CommitID) GetDate(c *Client) (time.Time, error) {
	if cached, ok := ancestorDateCache[cmt]; ok {
		return cached, nil
	}

	if ancestorDateCache == nil {
		ancestorDateCache = make(map[CommitID]time.Time)
	}

	obj, err := c.GetCommitObject(cmt)
	if err != nil {
		return time.Time{}, err
	}
	authorStr := obj.GetHeader("author")

	if authorStr == "" {
		return time.Time{}, fmt.Errorf("Commit %s does not have an author", cmt)
	}

	// authorStr is in the format:
	//    John Smith <jsmith@example.com> unixtime timezone
	//
	// we just split on space and rejoin the use the second last element
	// to get the date/time. It's not very elegant, and it discards time
	// zone information since time.Unix() doesn't allow us to specify time
	// zone and there's no way to set it on a time after the fact.
	authorPieces := strings.Split(authorStr, " ")
	if len(authorPieces) < 3 {
		return time.Time{}, fmt.Errorf("Could not parse author %s", authorStr)

	}

	unixTime, err := strconv.ParseInt(authorPieces[len(authorPieces)-2], 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	t := time.Unix(unixTime, 0)
	timeZone := authorPieces[len(authorPieces)-1]
	if loc, ok := tzCache[timeZone]; ok {
		return t.In(loc), nil
	}
	tzHours, err := strconv.ParseInt(timeZone[:len(timeZone)-2], 10, 64)
	if err != nil {
		panic(err)
	}
	loc := time.FixedZone(timeZone, int(tzHours*60*60))
	if tzCache == nil {
		tzCache = make(map[string]*time.Location)

	}
	tzCache[timeZone] = loc
	date := t.In(loc)

	ancestorDateCache[cmt] = date

	return date, nil

}

func (cmt CommitID) GetCommitMessage(c *Client) (CommitMessage, error) {
	obj, err := c.GetCommitObject(cmt)
	if err != nil {
		return "", err
	}
	for i, c := range obj.content {
		if c == '\n' && obj.content[i+1] == '\n' {
			return CommitMessage(string(obj.content[i+2:])), nil
		}
	}
	return "", nil
}

// Returns the author of the commit (with no time information attached) to
// the person object.
func (cmt CommitID) GetAuthor(c *Client) (Person, error) {
	obj, err := c.GetCommitObject(cmt)
	if err != nil {
		return Person{}, err
	}
	authorStr := obj.GetHeader("author")

	if authorStr == "" {
		return Person{}, fmt.Errorf("Could not parse author %s from commit %s", authorStr, cmt)
	}

	// authorStr is in the format:
	//    John Smith <jsmith@example.com> unixtime timezone
	authorPieces := strings.Split(authorStr, " ")
	if len(authorPieces) < 3 {
		return Person{}, fmt.Errorf("Commit %s does not have an author", cmt)
	}

	// FIXME: This should use
	email := authorPieces[len(authorPieces)-3]
	email = email[1 : len(email)-1] // strip off the < >
	author := strings.Join(authorPieces[:len(authorPieces)-3], " ")
	return Person{author, email, nil}, nil

}

// Returns the committer of the commit.
func (cmt CommitID) GetCommitter(c *Client) (Person, error) {
	obj, err := c.GetCommitObject(cmt)
	if err != nil {
		return Person{}, err
	}
	committerStr := obj.GetHeader("committer")

	if committerStr == "" {
		return Person{}, fmt.Errorf("Could not parse committer %s from commit %s", committerStr, cmt)
	}

	// committerStr is in the format:
	//    John Smith <jsmith@example.com> unixtime timezone
	committerPieces := strings.Split(committerStr, " ")
	if len(committerPieces) < 3 {
		return Person{}, fmt.Errorf("Commit %s does not have a committer", cmt)
	}

	// FIXME: This should use
	email := committerPieces[len(committerPieces)-3]
	email = email[1 : len(email)-1] // strip off the < >
	committer := strings.Join(committerPieces[:len(committerPieces)-3], " ")
	return Person{committer, email, nil}, nil

}

func (s CommitID) Ancestors(c *Client) ([]CommitID, error) {
	ancestors, err := s.ancestors(c)
	if err != nil {
		return nil, err
	}
	if ancestorDateCache == nil {
		ancestorDateCache = make(map[CommitID]time.Time)

	}

	sort.Slice(ancestors, func(i, j int) bool {
		var iDate, jDate time.Time
		var ok bool
		if ancestors[i].IsAncestor(c, ancestors[j]) {
			return false
		}
		if ancestors[j].IsAncestor(c, ancestors[i]) {
			return true
		}
		if iDate, ok = ancestorDateCache[ancestors[i]]; !ok {
			var err error
			iDate, err = ancestors[i].GetDate(c)
			if err != nil {
				panic(err)
			}
			ancestorDateCache[ancestors[i]] = iDate
		}
		if jDate, ok = ancestorDateCache[ancestors[j]]; !ok {
			var err error
			jDate, err = ancestors[j].GetDate(c)
			if err != nil {
				panic(err)
			}
			ancestorDateCache[ancestors[j]] = jDate
		}
		return jDate.Before(iDate)
	})
	return ancestors, nil
}

var ancestorCache map[CommitID][]CommitID
var ancestorDateCache map[CommitID]time.Time

func (s CommitID) ancestors(c *Client) (commits []CommitID, err error) {
	if cached, ok := ancestorCache[s]; ok {
		return cached, nil
	}

	parents, err := s.Parents(c)
	if err != nil {
		return nil, err
	}
	commits = append(commits, s)

	for _, p := range parents {
		grandparents, err := p.ancestors(c)
		if err != nil {
			return nil, err
		}

	duplicateCheck:
		for _, gp := range grandparents {
			for _, cmt := range commits {
				if cmt == gp {
					continue duplicateCheck
				}
			}
			commits = append(commits, gp)
		}
	}

	if ancestorCache == nil {
		ancestorCache = make(map[CommitID][]CommitID)
	}

	ancestorCache[s] = commits
	return
}

func NearestCommonParent(c *Client, com, other Commitish) (CommitID, error) {
	s, err := com.CommitID(c)
	if err != nil {
		return CommitID{}, err
	}
	ancestors, err := s.Ancestors(c)
	if err != nil {
		return CommitID{}, err
	}
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
	return c.GetAllObjectsExcept(cl, nil)
}

// returns a list of all objects in c excluding those in excludeList. It also populates excludeList
// with any objects encountered.
func (c CommitID) GetAllObjectsExcept(cl *Client, excludeList map[Sha1]struct{}) ([]Sha1, error) {
	var objects []Sha1
	tree, err := c.TreeID(cl)
	if err != nil {
		return nil, err
	}
	if excludeList != nil {
		if _, ok := excludeList[Sha1(tree)]; ok {
			return nil, nil
		}
	}
	objects = append(objects, Sha1(tree))
	children, err := tree.GetAllObjectsExcept(cl, excludeList, "", true, false)
	if err != nil {
		return nil, err
	}
	for _, s := range children {
		objects = append(objects, s.Sha1)
	}
	excludeList[Sha1(c)] = struct{}{}
	return objects, nil

}

func (c CommitID) getRefNamesList(cl *Client) (string, error) {
	// FIXME this needs to include other refs that aren't heads or tags
	refs, err := ShowRef(cl, ShowRefOptions{Heads: true, Tags: true}, []string{})
	if err != nil {
		return "", err
	}

	headRefSpec, err := SymbolicRefGet(cl, SymbolicRefOptions{}, "HEAD")
	if err != nil {
		return "", err
	}

	refsForCommit := ""
	for _, ref := range refs {
		if len(refsForCommit) != 0 {
			refsForCommit = refsForCommit + ", "
		}

		if ref.Value == Sha1(c) {

			// TODO relate any other top-level refs to their linked ref, not just HEAD
			if headRefSpec.String() == ref.Name {
				refsForCommit = refsForCommit + "HEAD -> "
			}
			refsForCommit = refsForCommit + ref.RefName()
		}
	}

	return refsForCommit, nil
}

func (c CommitID) FormatMedium(cl *Client) (string, error) {
	output := ""

	author, err := c.GetAuthor(cl)
	if err != nil {
		return "", err
	}
	output = output + fmt.Sprintf("commit %s", c)
	refNameList, _ := c.getRefNamesList(cl)
	if refNameList != "" {
		output = output + fmt.Sprintf(" (%s)", refNameList)
	}
	output = output + "\n"
	if parents, err := c.Parents(cl); len(parents) > 1 && err == nil {
		output = output + "Merge: "
		for i, p := range parents {
			output = output + fmt.Sprintf("%s", p)
			if i != len(parents)-1 {
				output = output + " "
			}
		}
		output = output + "\n"
	}
	date, err := c.GetDate(cl)
	if err != nil {
		return "", err
	}
	output = output + fmt.Sprintf("Author: %v\nDate:   %v\n\n", author, date.Format("Mon Jan 2 15:04:05 2006 -0700"))

	msg, err := c.GetCommitMessage(cl)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(msg.String()), "\n")
	for _, l := range lines {
		output = output + fmt.Sprintf("    %v\n", l)
	}
	output = output + "\n"
	return output, nil
}

func (c CommitID) Format(cl *Client, format string) (string, error) {
	output := format

	// Newline
	if strings.Contains(output, "%n") {
		output = strings.Replace(output, "%n", "\n", -1)
	}

	// Full commit hash
	if strings.Contains(output, "%H") {
		output = strings.Replace(output, "%H", c.String(), -1)
	}

	// Committer date as unix timestamp
	if strings.Contains(output, "%ct") {
		date, err := c.GetCommitterDate(cl)
		if err != nil {
			return "", err
		}
		output = strings.Replace(output, "%ct", strconv.FormatInt(date.Unix(), 10), -1)
	}

	// Author date as unix timestamp
	if strings.Contains(output, "%at") {
		date, err := c.GetDate(cl)
		if err != nil {
			return "", err
		}
		output = strings.Replace(output, "%at", strconv.FormatInt(date.Unix(), 10), -1)
	}

	// Show the non-stylized ref names beside any relevant commit
	if strings.Contains(output, "%D") {
		refNameList, _ := c.getRefNamesList(cl)
		output = strings.Replace(output, "%D", refNameList, -1)
	}

	// TODO Add more formatting options (there are many)

	return output, nil
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
	return t.GetAllObjectsExcept(cl, nil, prefix, recurse, excludeself)
}

// GetAllObjectsExcept is like GetAllObjects, except that it excludes those objects in excludeList. It also
// populates excludeList with any objects encountered.
func (t TreeID) GetAllObjectsExcept(cl *Client, excludeList map[Sha1]struct{}, prefix IndexPath, recurse, excludeself bool) (map[IndexPath]TreeEntry, error) {
	if excludeList != nil {
		excludeList[Sha1(t)] = struct{}{}
	}
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
			if excludeList != nil {
				if _, ok := excludeList[sha]; ok {
					i += 20
					entryStart = i + 1
					continue
				}
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
					children, err := childTree.GetAllObjectsExcept(cl, excludeList, "", recurse, excludeself)
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
			case "120000":
				mode = ModeSymlink
			case "160000":
				mode = ModeCommit
			default:
				panic(fmt.Sprintf("Unsupported mode %v in tree %s", string(perm), t))
			}
			val[IndexPath(name)] = TreeEntry{
				Sha1:     sha,
				FileMode: mode,
			}
			i += 20
			entryStart = i + 1
			if excludeList != nil {
				excludeList[sha] = struct{}{}
			}
		}
	}
	if excludeList != nil {
		excludeList[Sha1(t)] = struct{}{}
	}
	return val, nil
}

func (c CommitID) TreeID(cl *Client) (TreeID, error) {
	obj, err := cl.GetCommitObject(c)
	if err != nil {
		return TreeID{}, err
	}
	treeStr := obj.GetHeader("tree")
	s, err := Sha1FromString(treeStr)
	return TreeID(s), err
}

// Ensures the Tree implements Treeish
func (t TreeID) TreeID(cl *Client) (TreeID, error) {
	// Validate that it's a tree
	if Sha1(t).Type(cl) != "tree" {
		return TreeID{}, fmt.Errorf("Invalid tree: %v", t)
	}
	return t, nil
}

// Converts the Tree into an IndexEntries, to simplify comparisons between
// Trees and Indexes
func GetIndexMap(c *Client, t Treeish) (IndexMap, error) {
	indexentries, err := expandGitTreeIntoIndexes(c, t, true, false, false)
	if err != nil {
		return nil, err
	}
	// Create a fake index, to use the GetMap() function
	idx := &Index{Objects: indexentries}
	return idx.GetMap(), nil
}
