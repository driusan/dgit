package git

import (
	"regexp"
)

// Describes the options that may be specified on the command line for
// "git diff-index". Note that only raw mode is currently supported, even
// though all the other options are parsed/set in this struct.
type DiffIndexOptions struct {
	DiffCommonOptions

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

func DiffIndex(c *Client, opt *DiffIndexOptions, tree Treeish, paths []string) ([]HashDiff, error) {
	t, err := tree.TreeID(c)
	if err != nil {
		return nil, err
	}

	treeObjects, err := t.GetAllObjects(c, "", true, true)
	if err != nil {
		return nil, err
	}

	var val []HashDiff
	index, _ := c.GitDir.ReadIndex()

	for _, entry := range index.Objects {
		f, err := entry.PathName.FilePath(c)
		treeSha, ok := treeObjects[entry.PathName]
		fssha, _, err := HashFile("blob", f.String())
		if err != nil {
			return nil, err
		}

		if entry.Sha1 != fssha {
			val = append(val, HashDiff{entry.PathName, treeObjects[entry.PathName], TreeEntry{Sha1: Sha1{}, FileMode: ModeBlob}})
		} else if !ok {
			val = append(val, HashDiff{entry.PathName, TreeEntry{}, TreeEntry{Sha1: entry.Sha1, FileMode: entry.Mode}})
		} else if entry.Sha1 != treeSha.Sha1 {
			val = append(val, HashDiff{entry.PathName, treeObjects[entry.PathName], TreeEntry{Sha1: entry.Sha1, FileMode: entry.Mode}})
		} else {
			if err != nil {
				return nil, err
			}
		}
	}
	return val, nil
}
