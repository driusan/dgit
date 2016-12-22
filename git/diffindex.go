package git

import (
	"fmt"
	"regexp"
)

// Describes the options that may be specified on the command line for
// "git diff-index". Note that only raw mode is currently supported, even
// though all the other options are parsed/set in this struct.
type DiffIndexOptions struct {
	Patch bool

	// The 0 value implies 3.
	NumContextLines int

	Raw bool

	// Unimplemented. Probably never will be.
	CompactionHeuristic bool

	// Can be "default", "myers", "minimal", "patience", or "histogram"
	DiffAlgorithm string

	StatWidth, StatNameWidth, StatCount int
	NumStat                             bool
	ShortStat                           bool

	DirStat string

	Summary bool

	NullTerminate bool

	NameOnly, NameStatus bool

	Submodule string

	// Colour can have three states: "always" (true), "never" (false), or "auto" (nil)
	Color *bool

	// "color", "plain", "porcelain", or "none"
	WordDiff string

	WordDiffRegex *regexp.Regexp

	NoRenames bool

	// Warn if changes introduce conflict markers or whitespace errors.
	Check bool

	// Valid options in the []string are "old", "new", or "context"
	WhitespaceErrorHighlight []string

	FullIndex, Binary bool

	// Number of characters to abbreviate the hexadecimal object name to.
	Abbrev int

	// And 6 million more options, which are mostly for the unsupported patch
	// format anyways.
}

// A HashDiff represents a single line in a git diff-index type output.
type HashDiff struct {
	Name     IndexPath
	Src, Dst Sha1
}

func (h HashDiff) String() string {
	if h.Src == h.Dst {
		return ""
	}
	return fmt.Sprintf(":%v %v %v %v %v	%v", 0, 0, h.Src, h.Dst, "?", h.Name)
}
func DiffIndex(c *Client, opt *DiffIndexOptions, tree Treeish, paths []string) ([]HashDiff, error) {
	t, err := tree.TreeID(c)
	if err != nil {
		return nil, err
	}

	treeObjects, err := t.GetAllObjects(c, "")
	if err != nil {
		return nil, err
	}

	var val []HashDiff
	index, _ := c.GitDir.ReadIndex()

	for _, entry := range index.Objects {
		if entry.Sha1 != treeObjects[entry.PathName] {
			val = append(val, HashDiff{entry.PathName, treeObjects[entry.PathName], entry.Sha1})
		}
	}
	/*for name, o := range treeObjects {
		i := index.GetSha1(name)
		if i != o {
		}
	}*/
	return val, nil
}
