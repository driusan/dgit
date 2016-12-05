package main

import (
	"fmt"
	"os"

	libgit "github.com/driusan/git"
)

func LsTree(c *Client, repo *libgit.Repository, args []string) {
	commitId := args[0]
	tree, err := expandTreeIntoIndexesById(c, repo, commitId)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	for _, entry := range tree {
		// expandTree expands tree-type objects, so we just assume
		// everything is a blob. ie. This is always ls-tree -r
		fmt.Printf("%o blob %x\t%s\n", entry.Mode, entry.Sha1, entry.PathName)
	}
}

func expandTreeIntoIndexesById(c *Client, repo *libgit.Repository, treeId string) ([]*GitIndexEntry, error) {
	expanded := getTreeishId(c, repo, treeId)
	com, err := repo.GetCommit(expanded)
	if err != nil {
		return nil, err
	}
	tId := com.TreeId()
	tree := libgit.NewTree(repo, tId)
	if tree == nil {
		panic("Error retriving tree for commit")
	}
	return expandGitTreeIntoIndexes(repo, tree, "")

}
func expandGitTreeIntoIndexes(repo *libgit.Repository, tree *libgit.Tree, prefix string) ([]*GitIndexEntry, error) {
	newEntries := make([]*GitIndexEntry, 0)

	for _, entry := range tree.ListEntries() {
		var dirname string
		if prefix == "" {
			dirname = entry.Name()
		} else {
			dirname = prefix + "/" + entry.Name()
		}
		switch entry.Type {
		case libgit.ObjectBlob:
			newEntry := GitIndexEntry{}
			newEntry.Sha1 = entry.Id
			// This isn't right.
			// This should be:
			// "32-bit mode, split into:
			//      4-bit object type: valid values in binary are
			//          1000 (regular file), 1010 (symbolic link), and
			//          1110 (gitlink)
			//      3-bit unused
			//      9-bit unix permission. Only 0755 and 0644 are valid
			//          for regular files, symbolic links have 0 in this
			//          field"

			//go-gits entry mode is an int, but it needs to be a uint32
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
			/* I don't know how git can extract these values
			   from a tree. For now, leave them empty

			   Ctime     uint32
			   Ctimenano uint32
			   Dev uint32
			   Ino uint32
			   Uid uint32
			   Gid uint32
			*/
			newEntries = append(newEntries, &newEntry)
		case libgit.ObjectTree:

			subTree, err := tree.SubTree(entry.Name())
			if err != nil {
				panic(err)
			}
			subindexes, err := expandGitTreeIntoIndexes(repo, subTree, dirname)
			if err != nil {
				panic(err)
			}
			newEntries = append(newEntries, subindexes...)

		default:
			fmt.Fprintf(os.Stderr, "Unknown object type: %s\n", entry.Type)
		}
	}
	return newEntries, nil

}
