package git

import (
	"sort"
)

// Converts a git Tree-ish object to a slice of IndexEntrys.
// The most significant difference between trees and indexes are that indexes
// have fully expanded paths for all children, while trees only have their direct
// descendants available.
//
// If recurse is true, this will go into subtrees, and if not it will only expand
// direct descendants. If showTreeEntry is true, it will include a fake IndexEntry
// for the tree object (which is not valid in an index, but useful for commands
// like git ls-tree.)
//
// If treeOnly is true, expandGitTreeIntoIndexes will only retrieve metadata directly in the
// tree object, and not other data which is necessary for reading into an index (ie. file
// size)
func expandGitTreeIntoIndexes(c *Client, tree Treeish, recurse, showTreeEntry, treeOnly bool) ([]*IndexEntry, error) {
	sha1, err := tree.TreeID(c)
	if err != nil {
		return nil, err
	}
	t, err := sha1.TreeID(c)
	if err != nil {
		return nil, err
	}

	newEntries, err := expandGitTreeIntoIndexesRecursive(c, t, "", recurse, showTreeEntry, treeOnly)
	if err != nil {
		return nil, err
	}

	sort.Sort(ByPath(newEntries))
	return newEntries, nil
}

// This should not be called directly. It recurses into sub-trees to fully expand
// all child trees. After calling this function the IndexEntries are *not* sorted
// as the git index format requires.
func expandGitTreeIntoIndexesRecursive(c *Client, t TreeID, prefix string, recurse bool, showTreeEntry, treeOnly bool) ([]*IndexEntry, error) {
	vals, err := t.GetAllObjects(c, "", false, showTreeEntry)
	if err != nil {
		return nil, err
	}

	newEntries := make([]*IndexEntry, 0, len(vals))
	for path, treeEntry := range vals {
		var dirname IndexPath
		if prefix == "" {
			dirname = path
		} else {
			dirname = IndexPath(prefix) + "/" + path
		}
		if (treeEntry.FileMode != ModeTree) || showTreeEntry || !recurse {

			newEntry := IndexEntry{}
			newEntry.Sha1 = treeEntry.Sha1
			newEntry.Mode = treeEntry.FileMode
			newEntry.PathName = dirname

			if !treeOnly {
				// We need to read the object to see the size. It's
				// not in the tree.
				obj, err := c.GetObject(treeEntry.Sha1)
				if err != nil {
					return nil, err
				}
				newEntry.Fsize = uint32(obj.GetSize())

				// The git tree object doesn't include the mod time, so
				// we take the current mtime of the file (if possible)
				// for the index that we return.
				if fname, err := dirname.FilePath(c); err != nil {
					newEntry.Mtime = 0
				} else if fname.Exists() {
					if mtime, err := fname.MTime(); err == nil {
						newEntry.Mtime = mtime
					}
				}

				newEntry.Flags = uint16(len(dirname)) & 0xFFF
			}
			newEntries = append(newEntries, &newEntry)
		}
		if treeEntry.FileMode == ModeTree && recurse {
			subindexes, err := expandGitTreeIntoIndexesRecursive(c, TreeID(treeEntry.Sha1), dirname.String(), recurse, showTreeEntry, treeOnly)
			if err != nil {
				return nil, err
			}
			newEntries = append(newEntries, subindexes...)
		}
	}
	return newEntries, nil
}
