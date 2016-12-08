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

func getBranchId(repo *libgit.Repository, b string) (string, error) {
	return repo.GetCommitIdOfBranch(b)
}
func writeIndex(c *Client, idx *GitIndex, indexName string) error {
	if indexName == "" {
		return InvalidArgument
	}
	file, err := c.GitDir.Create(File(indexName))
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

func resetIndexFromCommit(c *Client, commitId string) error {
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return err
	}
	// If the index doesn't exist, idx is a new index, so ignore
	// the path error that ReadIndex is returning
	idx, _ := c.GitDir.ReadIndex()
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
	writeIndex(c, idx, "index")
	return nil
}

func resetWorkingTree(c *Client) error {
	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		return err
	}
	for _, indexEntry := range idx.Objects {
		obj, err := c.GetObject(indexEntry.Sha1)
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

func requiresGitDir(cmd string) bool {
	switch cmd {
	case "init", "clone":
		return false
	default:
		return true
	}
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
	cmd := args[0]
	if err != nil && requiresGitDir(cmd) {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(3)
	}
	if c != nil && c.GitDir == "" && requiresGitDir(cmd) {
		fmt.Fprintf(os.Stderr, "Could not find .git directory\n", err)
		os.Exit(4)
	}

	if len(args) > 1 {
		args = args[1:]
	}

	// TODO: Get rid of this. It's only here for a transition.
	var repo *libgit.Repository
	if c != nil {
		repo, _ = libgit.OpenRepository(c.GitDir.String())
	}
	switch cmd {
	case "init":
		Init(c, args)
	case "branch":
		Branch(c, repo, args)
	case "checkout":
		Checkout(c, args)
	case "add":
		Add(c, args)
	case "commit":
		sha1, err := Commit(c, repo, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		} else {
			fmt.Printf("%s\n", sha1)
		}
	case "commit-tree":
		sha1, err := CommitTree(c, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		} else {
			fmt.Printf("%s\n", sha1)
		}
	case "write-tree":
		sha1 := WriteTree(c, repo)
		fmt.Printf("%s\n", sha1)
	case "update-ref":
		UpdateRef(c, args)
	case "log":
		Log(c, repo, args)
	case "symbolic-ref":
		val := SymbolicRef(c, args)
		fmt.Printf("%s\n", val)
	case "clone":
		Clone(c, args)
	case "config":
		Config(c, args)
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
		HashObject(c, args)
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
