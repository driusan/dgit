package main

import (
	"errors"
	"fmt"
	libgit "github.com/driusan/git"
	"io/ioutil"
	"os"
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

func WriteTree(repo *libgit.Repository) {
	idx, _ := ReadIndex(repo)
	idx.WriteTree(repo)
}

func Config(repo *libgit.Repository, args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: go-git config [<options>]\n")
		return
	}
	file, err := os.OpenFile(repo.Path+"/config", os.O_RDWR, 0644)
	if err != nil {
		panic("Couldn't open config\n")
	}
	defer file.Close()

	config := parseConfig(repo, file)
	switch args[0] {
	case "--get":
		fmt.Printf("%s\n", config.GetConfig(args[1]))
		return
	case "--set":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Missing value to set config to\n")
			return
		}
		file.Seek(0, 0)
		config.SetConfig(args[1], args[2])
		config.WriteFile(file)
		return

	}
	panic("Unhandled action" + args[0])
}
func Fetch(repo *libgit.Repository, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Missing repository to fetch")
		return
	}

	file, err := os.Open(repo.Path + "/config")
	if err != nil {
		panic("Couldn't open config\n")
	}
	defer file.Close()
	config := parseConfig(repo, file)
	repoid := config.GetConfig("remote." + args[0] + ".url")
	var ups uploadpack
	if repoid[0:7] == "http://" || repoid[0:8] == "https://" {
		ups = smartHTTPServerRetriever{location: repoid,
			repo: repo,
		}
	} else {
		fmt.Fprintln(os.Stderr, "Unknown protocol.")
		return
	}
	refs, pack, err := ups.NegotiatePack()
	if err != nil {
		panic(err)
	}
	defer pack.Close()
	defer os.RemoveAll(pack.Name())
	pack.Seek(0, 0)
	fmt.Printf("Unpacking into %s\n", repo.Path)
	unpack(repo, pack)
	for _, ref := range refs {
		if repo.Path != "" {
			refloc := fmt.Sprintf("%s/%s", repo.Path, strings.TrimSpace(ref.Refname))
			refloc = strings.TrimSpace(refloc)
			fmt.Printf("Creating %s with %s", refloc, ref.Sha1)
			ioutil.WriteFile(
				refloc,
				[]byte(ref.Sha1),
				0644,
			)
		}
	}
}
func Checkout(repo *libgit.Repository, args []string) {
	switch len(args) {
	case 0:
		fmt.Fprintf(os.Stderr, "Missing argument for checkout")
		return
	}

	idx, _ := ReadIndex(repo)
	for _, file := range args {
		fmt.Printf("Doing something with %s\n", file)
		if _, err := os.Stat(file); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "File %s does not exist.\n")
			continue
		}
		for _, idxFile := range idx.Objects {
			if idxFile.PathName == file {
				obj, err := GetObject(repo, idxFile.Sha1)
				if err != nil {
					panic("Couldn't load object referenced in index.")
				}

				fmode := os.FileMode(idxFile.Mode)
				err = ioutil.WriteFile(file, obj.GetContent(), fmode)
				if err != nil {
					panic("Couldn't write file" + file)
				}
				os.Chmod(file, os.FileMode(idxFile.Mode))
			}
		}

	}
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
func Add(repo *libgit.Repository, args []string) {
	gindex, _ := ReadIndex(repo)
	for _, arg := range args {
		if _, err := os.Stat(arg); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "File %s does not exist.\n")
			continue
		}
		if file, err := os.Open(arg); err == nil {
			gindex.AddFile(repo, file)
		}
	}
	writeIndex(repo, gindex, "index")

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
	idx, err := ReadIndex(repo)
	if err != nil {
		return err
	}
	com, err := repo.GetCommit(commitId)
	if err != nil {
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
		err = ioutil.WriteFile(indexEntry.PathName, obj.GetContent(), os.FileMode(indexEntry.Mode))
		if err != nil {

			continue
		}
		os.Chmod(indexEntry.PathName, os.FileMode(indexEntry.Mode))

	}
	return nil
}

func Reset(repo *libgit.Repository, args []string) {
	commitId, err := getHeadId(repo)
	var resetPaths = false
	var mode string = "mixed"
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't find HEAD commit.\n")
	}
	for _, val := range args {
		if _, err := os.Stat(val); err == nil {
			resetPaths = true
			panic("TODO: I'm not prepared to handle git reset <paths>")
		}
		// The better way to do this would have been:
		// git reset [treeish] <paths>:
		//  stat val
		//      if valid file:
		//          reset index to status at [treeish]
		// (opposite of git add)
		//

		// Expand the parameter passed to a CommitID. We need
		// the CommitID that it refers to no matter what mode
		// we're in, but if we've already found a path already
		// then the time for a treeish option is past.
		if val[0] != '-' && resetPaths == false {
			commitId = getTreeishId(repo, val)
		} else {
			switch val {
			case "--soft":
				mode = "soft"
			case "--mixed":
				mode = "mixed"
			case "--hard":
				mode = "hard"
			default:
				fmt.Fprintf(os.Stderr, "Unknown option: %s", val)
			}
		}
	}
	if resetPaths == false {
		// no paths were found. This is the form
		//  git reset [mode] commit
		// First, update the head reference for all modes
		branchName := getHeadBranch(repo)
		err := ioutil.WriteFile(repo.Path+"/refs/heads/"+branchName,
			[]byte(fmt.Sprintf("%s", commitId)),
			0644,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error updating head reference: %s\n", err)
			return
		}

		// mode: soft: do not touch working tree or index
		//       mixed (default): reset the index but not working tree
		//       hard: reset the index and the working tree
		switch mode {
		case "soft":
			// don't do anything for soft reset other than update
			// the head reference
		case "hard":
			resetIndexFromCommit(repo, commitId)
			resetWorkingTree(repo)
		case "mixed":
			fallthrough
		default:
			resetIndexFromCommit(repo, commitId)
		}

	}
}
func Branch(repo *libgit.Repository, args []string) {
	switch len(args) {
	case 0:
		branches, err := repo.GetBranches()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not get list of branches.")
			return
		}
		head := getHeadBranch(repo)
		for _, b := range branches {
			if head == b {
				fmt.Print("* ")
			} else {
				fmt.Print("  ")
			}
			fmt.Println(b)
		}
	case 1:
		if head, err := getHeadId(repo); err == nil {
			if cerr := libgit.CreateBranch(repo.Path, args[0], head); cerr != nil {
				fmt.Fprintf(os.Stderr, "Could not create branch: %s\n", cerr.Error())
			}
		} else {
			fmt.Fprintf(os.Stderr, "Could not create branch: %s\n", err.Error())
		}
	default:
		fmt.Fprintln(os.Stderr, "Usage: go-git branch [branchname]")
	}

}
func Init(repo *libgit.Repository, args []string) {
	if len(args) > 0 {
		if dir := args[len(args)-1]; dir != "init" {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				panic("Couldn't create directory for initializing git.")
			}
			err = os.Chdir(dir)
			if err != nil {
				panic("Couldn't change working directory while initializing git.")
			}
			if repo != nil {
				repo.Path = ".git/"
			}
		}
	}
	// These are all the directories created by a clean "git init"
	// with the canonical git implementation
	os.Mkdir(".git", 0755)
	os.MkdirAll(".git/objects/pack", 0755)
	os.MkdirAll(".git/objects/info", 0755)
	os.MkdirAll(".git/info", 0755)  // Should have exclude file in it
	os.MkdirAll(".git/hooks", 0755) // should have sample hooks in it.
	os.MkdirAll(".git/branches", 0755)
	os.MkdirAll(".git/refs/heads", 0755)
	os.MkdirAll(".git/refs/tags", 0755)

	ioutil.WriteFile(".git/HEAD", []byte("ref: refs/heads/master\n"), 0644)
	ioutil.WriteFile(".git/config", []byte("[core]\n\trepositoryformatversion = 0\n\tbare = false\n"), 0644)
	ioutil.WriteFile(".git/description", []byte("Unnamed repository; edit this file 'description' to name the repository.\n"), 0644)

}

func Clone(repo *libgit.Repository, args []string) {
	var repoid string
	// TODO: This argument parsing should be smarter and more
	// in line with how cgit does it.
	switch len(args) {
	case 0:
		fmt.Fprintln(os.Stderr, "Usage: go-git clone repo [directory]")
		return
	case 1:
		repoid = args[0]
	default:
		repoid = args[0]
	}
	repoid = strings.TrimRight(repoid, "/")
	pieces := strings.Split(repoid, "/")

	var dirName string
	if len(pieces) > 0 {
		dirName = pieces[len(pieces)-1]
	}
	dirName = strings.TrimSuffix(dirName, ".git")

	if _, err := os.Stat(dirName); err == nil {
		fmt.Fprintf(os.Stderr, "Directory %s already exists, can not clone.\n", dirName)
		return
	}
	if dirName == "" {
		panic("No directory left to clone into.")
	}

	if repo == nil {
		repo = &libgit.Repository{}
	}

	Init(repo, []string{dirName})

	Config(repo, []string{"--set", "remote.origin.url", repoid})
	Config(repo, []string{"--set", "branch.master.remote", "origin"})

	Fetch(repo, []string{"origin"})
	Reset(repo, []string{"--hard"})
}

func main() {

	if len(os.Args) > 1 {
		repo, _ := libgit.OpenRepository(".git")
		switch os.Args[1] {
		case "init":
			Init(repo, os.Args[2:])
		case "branch":
			Branch(repo, os.Args[2:])
		case "checkout":
			Checkout(repo, os.Args[2:])
		case "add":
			Add(repo, os.Args[2:])
		case "write-tree":
			WriteTree(repo)
		case "clone":
			Clone(repo, os.Args[2:])
		case "config":
			Config(repo, os.Args[2:])
		case "fetch":
			Fetch(repo, os.Args[2:])

		case "reset":
			Reset(repo, os.Args[2:])
		}
	}
}
