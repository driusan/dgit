package git

import (
	"regexp"
	"sort"
)

// Describes the options that may be specified on the command line for
// "git diff-index". Note that only raw mode is currently supported, even
// though all the other options are parsed/set in this struct.
type DiffTreeOptions struct {
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

	// Recurse into subtrees.
	Recurse bool
	// And 6 million more options, which are mostly for the unsupported patch
	// format anyways.
}

func DiffTree(c *Client, opt *DiffTreeOptions, tree1, tree2 Treeish, paths []string) ([]HashDiff, error) {
	t1, err := tree1.TreeID(c)
	if err != nil {
		return nil, err
	}
	t2, err := tree2.TreeID(c)
	if err != nil {
		return nil, err
	}

	tree1Objects, err := t1.GetAllObjects(c, "", opt.Recurse, opt.Recurse)
	if err != nil {
		return nil, err
	}
	tree2Objects, err := t2.GetAllObjects(c, "", opt.Recurse, opt.Recurse)
	if err != nil {
		return nil, err
	}

	var val []HashDiff

	for name, sha := range tree1Objects {
		if osha := tree2Objects[name]; sha != osha {
			val = append(val, HashDiff{name, sha, osha, 0, 0})
		}
	}

	// Check for files that were added in tree2 but missing in tree1, which
	// would have gotten caught by the above ranging.
	for name, sha := range tree2Objects {
		if _, ok := tree1Objects[name]; !ok {
			val = append(val, HashDiff{name, TreeEntry{Sha1{}, 0}, sha, 0, 0})
		}
	}

	sort.Sort(ByName(val))

	return val, nil
}
