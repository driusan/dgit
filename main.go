package main

import (
	"errors"
	"flag"
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

func getBranchId(repo *libgit.Repository, b string) (string, error) {
	return repo.GetCommitIdOfBranch(b)
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

// FIXME: This should be removed. RevParse() is the correct thing to use.
func getTreeishId(c *Client, repo *libgit.Repository, treeish string) string {
	if treeish == "HEAD" {
		if head, err := c.GetHeadID(); err == nil {
			return head
		}
	}
	if branchId, err := repo.GetCommitIdOfBranch(treeish); err == nil {
		return branchId
	}
	if len(treeish) == 40 {
		return treeish
	}
	panic("TODO: Didn't implement getTreeishId")
}

func resetIndexFromCommit(c *Client, repo *libgit.Repository, commitId string) error {
	// If the index doesn't exist, idx is a new index, so ignore
	// the path error that ReadIndex is returning
	idx, _ := ReadIndex(c, repo)
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

func resetWorkingTree(c *Client, repo *libgit.Repository) error {
	idx, err := ReadIndex(c, repo)
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

func main() {
	workdir := flag.String("work-tree", "", "specify the working directory of git")
	gitdir := flag.String("git-dir", "", "specify the repository of git")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [global options] subcommand [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nGlobal options:\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(2)
	}
	c, err := NewClient(*gitdir, *workdir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(3)
	}
	if c.GitDir == "" {
		fmt.Fprintf(os.Stderr, "Could not find .git directory\n", err)
		os.Exit(4)
	}
	cmd := args[0]
	args = args[1:]

	// TODO: Get rid of this. It's only here for a transition.
	repo, _ := libgit.OpenRepository(c.GitDir.String())
	switch cmd {
	case "init":
		Init(repo, args)
	case "branch":
		Branch(c, repo, args)
	case "checkout":
		Checkout(c, repo, args)
	case "add":
		Add(c, repo, args)
	case "commit":
		sha1 := Commit(c, repo, args)
		fmt.Printf("%s\n", sha1)
		fmt.Printf("%s\n", sha1)
	case "commit-tree":
		sha1 := CommitTree(repo, args)
		fmt.Printf("%s\n", sha1)
	case "write-tree":
		sha1 := WriteTree(c, repo)
		fmt.Printf("%s\n", sha1)
	case "update-ref":
		UpdateRef(repo, args)
	case "log":
		Log(c, repo, args)
	case "symbolic-ref":
		val := SymbolicRef(repo, args)
		fmt.Printf("%s\n", val)
	case "clone":
		Clone(c, repo, args)
	case "config":
		Config(repo, args)
	case "fetch":
		Fetch(c, repo, args)
	case "reset":
		Reset(c, repo, args)
	case "merge":
		Merge(c, repo, args)
	case "rev-parse":
		commits, err := RevParse(c, repo, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
		for _, sha := range commits {
			if sha.Excluded {
				fmt.Print("^")
			}
			fmt.Println(sha.Id.String())
		}

	case "rev-list":
		RevList(c, repo, args)
	case "hash-object":
		HashObject(repo, args)
	case "status":
		Status(c, repo, args)
	case "ls-tree":
		LsTree(c, repo, args)
	case "push":
		Push(c, repo, args)
	case "pack-objects":
		PackObjects(repo, os.Stdin, args)
	case "send-pack":
		SendPack(repo, args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown git command %s.\n", cmd)
	}
}
