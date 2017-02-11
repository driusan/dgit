package git

import (
	"regexp"
	"sort"
)

// Describes the options that may be specified on the command line for
// "git diff-files". Note that only raw mode is currently supported, even
// though all the other options are parsed/set in this struct.
type DiffFilesOptions struct {
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

// DiffFiles implements the git diff-files command.
// It compares the file system to the index.
func DiffFiles(c *Client, opt *DiffFilesOptions, paths []string) ([]HashDiff, error) {
	indexentries, err := LsFiles(
		c,
		&LsFilesOptions{
			Cached: true, Deleted: true, Modified: true, Others: true,
		},
		paths,
	)
	if err != nil {
		return nil, err
	}

	var val []HashDiff

	for _, idx := range indexentries {
		fs := TreeEntry{}
		idxtree := TreeEntry{idx.Sha1, idx.Mode}

		f, err := idx.PathName.FilePath(c)
		if err != nil || !f.Exists() {
			// If there was an error, treat it as a non-existant file
			// and just use the empty Sha1
			val = append(val, HashDiff{idx.PathName, idxtree, fs})
			continue
		}
		stat, err := f.Stat()
		if err != nil {
			val = append(val, HashDiff{idx.PathName, idxtree, fs})
			continue
		}

		switch {
		case stat.Mode().IsDir():
			fs.FileMode = ModeTree
		case !stat.Mode().IsRegular():
			// FIXME: This doesn't take into account that the file
			// might be some kind of non-symlink non-regular file.
			fs.FileMode = ModeSymlink
		case stat.Mode().Perm()&0100 != 0:
			fs.FileMode = ModeExec
		default:
			fs.FileMode = ModeBlob
		}
		fs.Sha1, _, err = HashFile("blob", f.String())
		if fs != idxtree {
			val = append(val, HashDiff{idx.PathName, idxtree, fs})
		}
	}

	sort.Sort(ByName(val))

	return val, nil
}
