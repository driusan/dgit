package git

import (
	//	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Options for LsTree. (These are almost all presentational and
// don't affect the behaviour of LsTree itself, but are in this
// struct to make it easier to use the flags package to parse the
// command line.
type LsTreeOptions struct {
	TreeOnly  bool
	Recurse   bool
	ShowTrees bool
	Long      bool

	NullTerminate bool

	NameOnly bool

	Abbrev int

	FullName bool
	FullTree bool
}

// Implements the git ls-tree subcommand. This will return a list of
// index entries in tree that match paths
func LsTree(c *Client, opts LsTreeOptions, tree Treeish, paths []File) ([]*IndexEntry, error) {
	// This is a really inefficient implementation, but the output (seems to)
	// match the official git client in most cases.
	//
	// At some point it might be worth looking into how the real git client
	// does it to steal their algorithm, but for now this works.

	// Start by completely expanding tree, and looking for the subtrees
	// which match the directories of paths, but first check if we're
	// in the root of the workgir (or using FullTree), in which case we
	// can just use the tree passed.
	if c.IsBare() {
		opts.FullTree = true
	}
	if opts.FullTree {
		return lsTree(c, opts, tree, "", paths)
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	abspwd, err := filepath.Abs(wd)
	if err != nil {
		return nil, err
	}
	abswd, err := filepath.Abs(c.WorkDir.String())
	if err != nil {
		return nil, err
	}
	if abspwd == abswd {
		// Root dir, don't bother trying to find an appropriate subtree.
		return lsTree(c, opts, tree, "", paths)
	}

	// Find all the subtrees that exist anywhere.
	allentries, err := expandGitTreeIntoIndexes(c, tree, true, true, true)
	if err != nil {
		return nil, err
	}

	var vals []*IndexEntry
	for _, entry := range allentries {
		f, err := entry.PathName.FilePath(c)
		if err != nil {
			return nil, err
		}

		if len(paths) == 0 {
			// There were no paths specified, so it's safe to just
			// return lsTree once we found the proper subtree.
			if f.String() == "." {
				return lsTree(c, opts, TreeID(entry.Sha1), entry.PathName.String(), paths)
			}
		} else {
			// Go through each path, and check if it matches
			for _, path := range paths {
				ipath, err := path.IndexPath(c)
				if err != nil {
					return nil, err
				}
				if ipath == entry.PathName {
					if entry.Mode == ModeTree {
						// It was explicitly asked for,
						// so add the tree entry, unless
						// -r was specified.
						// The official git client will
						// also recurse into a path that
						// ends with a /, even without
						// -r.
						if opts.Recurse || strings.HasSuffix(path.String(), "/") {
							if opts.ShowTrees {
								// -r was specified, but so was -t
								vals = append(vals, entry)
							}

							dir, err := lsTree(c, opts, TreeID(entry.Sha1), entry.PathName.String(), paths)
							if err != nil {
								return nil, err
							}
							vals = append(vals, dir...)
						} else {
							// It was an explicit match, so add it.
							vals = append(vals, entry)
						}
					} else {
						// It was an explicit match against
						// something that isn't a tree, add it.
						vals = append(vals, entry)
					}

				} else if ipath == entry.PathName+"/" {
					// The official git client will recurse
					// into a directory that ends with /,
					// even without --recurse.
					if entry.Mode == ModeTree {
						if opts.ShowTrees {
							// -r was specified, but so was -t
							vals = append(vals, entry)
						}

						dir, err := lsTree(c, opts, TreeID(entry.Sha1), entry.PathName.String(), paths)
						if err != nil {
							return nil, err
						}
						vals = append(vals, dir...)

					}
				}
			}
		}
	}

	// Finally, sort them and remove duplicates, since we were lazy above
	sort.Sort(ByPath(vals))
	// We're also lazy here, but we already conceded that this implementation
	// was inefficient a long time ago..
	retvals := make([]*IndexEntry, 0, len(vals))
	for _, v := range vals {
		if len(retvals) == 0 {
			retvals = append(retvals, v)
			continue
		}
		if v.PathName != retvals[len(retvals)-1].PathName {
			retvals = append(retvals, v)
		}
	}
	return retvals, nil
}

func lsTree(c *Client, opts LsTreeOptions, tree Treeish, prefix string, paths []File) ([]*IndexEntry, error) {
	entries, err := expandGitTreeIntoIndexes(c, tree, opts.Recurse, opts.ShowTrees, true)
	if err != nil {
		return nil, err
	}
	filtered := make([]*IndexEntry, 0, len(entries))
	for _, entry := range entries {
		if prefix != "" {
			entry.PathName = IndexPath(prefix) + "/" + entry.PathName
		}
		f, err := entry.PathName.FilePath(c)
		if err != nil {
			return nil, err
		}
		if fpath := f.String(); strings.HasPrefix(fpath, "../") || len(paths) > 0 {
			skip := true
			if len(paths) == 0 && opts.FullTree {
				skip = false
			}
			for _, explicit := range paths {
				exp := explicit.String()
				if opts.FullTree {
					exp = c.WorkDir.String() + "/" + exp
				}
				eAbs, err := filepath.Abs(exp)
				if err != nil {
					return nil, err
				}
				fAbs, err := filepath.Abs(fpath)
				if err != nil {
					return nil, err
				}
				if strings.HasPrefix(fAbs, eAbs) {
					skip = false
					break
				}
			}
			if skip {
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	return filtered, nil
}
