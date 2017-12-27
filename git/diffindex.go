package git

// Describes the options that may be specified on the command line for
// "git diff-index". Note that only raw mode is currently supported, even
// though all the other options are parsed/set in this struct.
type DiffIndexOptions struct {
	DiffCommonOptions

	// Only compare the index to the tree, not the filesystem.
	Cached bool
}

func DiffIndex(c *Client, opt DiffIndexOptions, tree Treeish, paths []File) ([]HashDiff, error) {
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
		if err != nil {
			return nil, err
		}
		treeSha, ok := treeObjects[entry.PathName]
		var fssha Sha1
		mode := ModeBlob
		if !opt.Cached {
			fssha, _, err = HashFile("blob", f.String())
			if err != nil {
				mode = 0
			}
			// err means file was deleted, which isn't really an error, so ignore
			// it.
		} else {
			fssha = entry.Sha1
		}

		if entry.Sha1 != fssha {
			val = append(val, HashDiff{entry.PathName, treeObjects[entry.PathName], TreeEntry{Sha1: Sha1{}, FileMode: mode}})
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
