package main

import (
	"errors"
	"fmt"
	libgit "github.com/driusan/git"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var InvalidHead error = errors.New("Invalid HEAD")
var InvalidArgument error = errors.New("Invalid argument to function")

func getHeadBranch(repo *libgit.Repository) string {
	file, _ := os.Open(repo.Path + "/HEAD")
	value, _ := ioutil.ReadAll(file)
	if prefix := string(value[0:5]); prefix != "ref: " {
		panic("Could not understand HEAD pointer.")
	} else {
		ref := strings.Split(string(value[5:]), "/")
		if len(ref) != 3 {
			panic("Could not parse branch out of HEAD")
		}
		if ref[0] != "refs" || ref[1] != "heads" {
			panic("Unknown HEAD reference")
		}
		return strings.TrimSpace(ref[2])
	}
	return ""

}
func getHeadId(repo *libgit.Repository) (string, error) {
	if headBranch := getHeadBranch(repo); headBranch != "" {
		return repo.GetCommitIdOfBranch(getHeadBranch(repo))
	}
	return "", InvalidHead
}

func writeIndex(repo *libgit.Repository, idx *GitIndex, indexName string) error {
	if indexName == "" {
		return InvalidArgument
	}
	file, err := os.Create(repo.Path + "/" + indexName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not write index")
		return err
	}
	defer file.Close()
	idx.WriteIndex(file)
	return nil
}

func getTreeishId(repo *libgit.Repository, treeish string) string {
	if branchId, err := repo.GetCommitIdOfBranch(treeish); err == nil {
		return branchId
	}
	if len(treeish) == 40 {
		return treeish
	}
	panic("TODO: Didn't implement getTreeishId")
}

func resetIndexFromCommit(repo *libgit.Repository, commitId string) error {
	// If the index doesn't exist, idx is a new index, so ignore
	// the path error that ReadIndex is returning
	idx, _ := ReadIndex(repo)
	com, err := repo.GetCommit(commitId)
	if err != nil {
		fmt.Printf("%s\n", err)
		return err
	}
	treeId := com.TreeId()
	tree := libgit.NewTree(repo, treeId)
	if tree == nil {
		panic("Error retriving tree for commit")
	}
	idx.ResetIndex(repo, tree)
	writeIndex(repo, idx, "index")
	return nil
}

func resetWorkingTree(repo *libgit.Repository) error {
	idx, err := ReadIndex(repo)
	if err != nil {
		return err
	}
	for _, indexEntry := range idx.Objects {
		obj, err := GetObject(repo, indexEntry.Sha1)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not retrieve %x for %s: %s\n", indexEntry.Sha1, indexEntry.PathName, err)
			continue
		}
		if strings.Index(indexEntry.PathName, "/") > 0 {
			os.MkdirAll(filepath.Dir(indexEntry.PathName), 0755)
		}
		err = ioutil.WriteFile(indexEntry.PathName, obj.GetContent(), os.FileMode(indexEntry.Mode))
		if err != nil {

			continue
		}
		os.Chmod(indexEntry.PathName, os.FileMode(indexEntry.Mode))

	}
	return nil
}

func getGitDir() string {
	startPath, err := os.Getwd()
	if err != nil {
		return ""
	}
	if dirinfo, err := os.Stat(startPath + "/.git"); err == nil && dirinfo.IsDir() {
		return startPath + "/.git"
	}
	pieces := strings.Split(startPath, "/")

	for i := len(pieces); i > 0; i -= 1 {
		dir := strings.Join(pieces[0:i], "/")
		if dirinfo, err := os.Stat(dir + "/.git"); err == nil && dirinfo.IsDir() {
			return dir + "/.git"
		}
	}
	return ""
}
func main() {
	if len(os.Args) > 1 {
		repo, _ := libgit.OpenRepository(getGitDir())
		switch os.Args[1] {
		case "init":
			Init(repo, os.Args[2:])
		case "branch":
			Branch(repo, os.Args[2:])
		case "checkout":
			Checkout(repo, os.Args[2:])
		case "add":
			Add(repo, os.Args[2:])
		case "commit":
			sha1 := Commit(repo, os.Args[2:])
			fmt.Printf("%s\n", sha1)
			fmt.Printf("%s\n", sha1)
		case "commit-tree":
			sha1 := CommitTree(repo, os.Args[2:])
			fmt.Printf("%s\n", sha1)
		case "write-tree":
			sha1 := WriteTree(repo)
			fmt.Printf("%s\n", sha1)
		case "update-ref":
			UpdateRef(repo, os.Args[2:])
		case "log":
			Log(repo, os.Args[2:])
		case "symbolic-ref":
			val := SymbolicRef(repo, os.Args[2:])
			fmt.Printf("%s\n", val)
		case "clone":
			Clone(repo, os.Args[2:])
		case "config":
			Config(repo, os.Args[2:])
		case "fetch":
			Fetch(repo, os.Args[2:])
		case "reset":
			Reset(repo, os.Args[2:])
		case "merge":
			Merge(repo, os.Args[2:])
		case "rev-parse":
			RevParse(repo, os.Args[2:])
		default:
			fmt.Fprintf(os.Stderr, "Unknown git command.\n")
		}
	}
}
