package git

import (
//	"fmt"
)

// Describes the options that may be specified on the command line for
// "git diff-index". Note that only raw mode is currently supported, even
// though all the other options are parsed/set in this struct.
type DiffIndexOptions struct {
	DiffCommonOptions

	// Only compare the index to the tree, not the filesystem.
	Cached bool
}

func DiffIndex(c *Client, opt DiffIndexOptions, index *Index, tree Treeish, paths []File) ([]HashDiff, error) {
	lsTreeOpts := LsTreeOptions{Recurse: true}
	if len(paths) == 0 {
		lsTreeOpts.FullTree = true
	}

	treefiles, err := LsTree(c, lsTreeOpts, tree, paths)
	if err != nil {
		return nil, err
	}

	// This used to be tree.GetAllObjects. Convert the LsTree output from
	// a slice to a map, so that the loop doesn't need to iterate over
	// everything every time..
	treeObjects := make(map[IndexPath]struct {
		Tree TreeEntry
		Size uint
	})
	for _, path := range treefiles {
		treeObjects[path.PathName] = struct {
			Tree TreeEntry
			Size uint
		}{TreeEntry{path.Sha1, path.Mode}, uint(path.Fsize)}
	}

	var val []HashDiff

	for _, entry := range index.Objects {
		if len(paths) > 0 {
			if _, ok := treeObjects[entry.PathName]; !ok {
				continue
			}
		}
		f, err := entry.PathName.FilePath(c)
		if err != nil {
			return nil, err
		}
		treeSha, ok := treeObjects[entry.PathName]
		var fssha Sha1
		mode := ModeBlob
		var fsize uint
		if !opt.Cached {
			fssha1, data, err := HashFile("blob", f.String())
			if err != nil {
				// err means file was deleted, which isn't really an error, so ignore
				// it.
				mode = 0
				fsize = 0
			} else {
				fssha = fssha1
				fsize = uint(len(data))
			}
		} else {
			fssha = entry.Sha1
			fsize = uint(entry.Fsize)
		}

		if entry.Sha1 != fssha {
			val = append(val, HashDiff{entry.PathName, treeObjects[entry.PathName].Tree, TreeEntry{Sha1: Sha1{}, FileMode: mode}, treeObjects[entry.PathName].Size, 0})
		} else if !ok {
			val = append(val, HashDiff{entry.PathName, TreeEntry{}, TreeEntry{Sha1: entry.Sha1, FileMode: entry.Mode}, 0, fsize})
		} else if entry.Sha1 != treeSha.Tree.Sha1 {
			val = append(val, HashDiff{entry.PathName, treeSha.Tree, TreeEntry{Sha1: entry.Sha1, FileMode: entry.Mode}, treeSha.Size, fsize})
		} else {
			if err != nil {
				return nil, err
			}
		}
	}
	return val, nil
}
