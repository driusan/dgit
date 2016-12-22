package git

import (
	libgit "github.com/driusan/git"
)

// This should be refactored into something that makes more sense, but is currently
// exported for cmd.LsTree. You probably shouldn't use this directly.
func ExpandTreeIntoIndexesById(c *Client, treeId string, recurse, showTreeEntry bool) ([]*IndexEntry, error) {
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return nil, err
	}

	expanded, err := RevParse(c, RevParseOptions{}, []string{treeId})
	if err != nil {
		return nil, err
	}
	com, err := repo.GetCommit(expanded[0].Id.String())
	if err != nil {
		return nil, err
	}
	tId := com.TreeId()
	tree := libgit.NewTree(repo, tId)
	if tree == nil {
		panic("Error retriving tree for commit")
	}
	return expandGitTreeIntoIndexes(repo, tree, "", recurse, showTreeEntry)

}
func expandGitTreeIntoIndexes(repo *libgit.Repository, tree *libgit.Tree, prefix string, recurse bool, showTreeEntry bool) ([]*IndexEntry, error) {
	newEntries := make([]*IndexEntry, 0)
	for _, entry := range tree.ListEntries() {
		var dirname string
		if prefix == "" {
			dirname = entry.Name()
		} else {
			dirname = prefix + "/" + entry.Name()
		}
		if (entry.Type != libgit.ObjectTree) || showTreeEntry || !recurse {
			newEntry := IndexEntry{}
			sha, err := Sha1FromString(entry.Id.String())
			if err != nil {
				panic(err)
			}
			newEntry.Sha1 = sha
			switch entry.EntryMode() {
			case libgit.ModeBlob:
				newEntry.Mode = 0100644
			case libgit.ModeExec:
				newEntry.Mode = 0100755
			case libgit.ModeSymlink:
				newEntry.Mode = 0120000
			case libgit.ModeCommit:
				newEntry.Mode = 0160000
			case libgit.ModeTree:
				newEntry.Mode = 0040000
			}
			newEntry.PathName = dirname
			newEntry.Fsize = uint32(entry.Size())

			modTime := entry.ModTime()
			newEntry.Mtime = uint32(modTime.Unix())
			newEntry.Mtimenano = uint32(modTime.Nanosecond())
			newEntry.Flags = uint16(len(dirname)) & 0xFFF
			newEntries = append(newEntries, &newEntry)
		}
		if entry.Type == libgit.ObjectTree && recurse {
			subTree, err := tree.SubTree(entry.Name())
			if err != nil {
				panic(err)
			}
			subindexes, err := expandGitTreeIntoIndexes(repo, subTree, dirname, recurse, showTreeEntry)
			if err != nil {
				panic(err)
			}
			newEntries = append(newEntries, subindexes...)
		}

	}
	return newEntries, nil

}
