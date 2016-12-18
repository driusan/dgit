package main

import (
	"flag"
	"fmt"
	"os"

	libgit "github.com/driusan/git"
)

func LsTree(c *Client, repo *libgit.Repository, args []string) error {
	var treeonly, recursepaths, showtrees, endnil bool
	//var showlong1, showlong2 bool
	var nameonly, namestatus bool
	//flag.StringVar(&t, "t", "blob", "-t object type")
	flag.BoolVar(&treeonly, "d", true, "Show only the named tree, not its children")
	flag.BoolVar(&recursepaths, "r", false, "Recurse into sub-trees")
	flag.BoolVar(&showtrees, "t", false, "Show trees even when recursing into them")
	flag.BoolVar(&endnil, "z", false, "\\0 line termination on output")

	flag.BoolVar(&nameonly, "name-only", false, "Only show the names of the files")
	flag.BoolVar(&namestatus, "name-status", false, "Only show the names of the files")

	nameonly = nameonly || namestatus
	//showlong := (showlong1 || showlong2)

	/*
		flag.BoolVar(&showlong1, "l", false, "Show size of blob entries")
		flag.BoolVar(&showlong2, "long", false, "Show size of blob entries")
		//showlong := (showlong1 || showlong2)
	*/
	fakeargs := []string{"git-ls-tree"}
	os.Args = append(fakeargs, args...)

	flag.Parse()
	args = flag.Args()
	if len(args) < 1 {
		flag.Usage()
		return fmt.Errorf("Missing tree")
	}

	commitId, err := RevParse(c, []string{args[0]})
	if err != nil {
		return err
	}

	tree, err := expandTreeIntoIndexesById(c, repo, commitId[0].Id.String(), recursepaths, showtrees)
	if err != nil {
		return err
	}
	for _, entry := range tree {
		var lineend string
		if endnil {
			lineend = "\000"
		} else {
			lineend = "\n"
		}
		if !nameonly {
			fmt.Printf("%0.6o %s %s\t%s%s", entry.Mode, entry.Sha1.Type(c), entry.Sha1, entry.PathName, lineend)
		} else {
			fmt.Printf("%s%s", entry.PathName, lineend)
		}
	}
	return nil
}

func expandTreeIntoIndexesById(c *Client, repo *libgit.Repository, treeId string, recurse, showTreeEntry bool) ([]*GitIndexEntry, error) {
	expanded, err := RevParse(c, []string{treeId})
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
func expandGitTreeIntoIndexes(repo *libgit.Repository, tree *libgit.Tree, prefix string, recurse bool, showTreeEntry bool) ([]*GitIndexEntry, error) {
	newEntries := make([]*GitIndexEntry, 0)
	for _, entry := range tree.ListEntries() {
		var dirname string
		if prefix == "" {
			dirname = entry.Name()
		} else {
			dirname = prefix + "/" + entry.Name()
		}
		if (entry.Type != libgit.ObjectTree) || showTreeEntry || !recurse {
			newEntry := GitIndexEntry{}
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
