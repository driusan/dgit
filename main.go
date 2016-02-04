package main

import (
	"errors"
	"fmt"
	libgit "github.com/gogits/git"
	"io/ioutil"
	"os"
	"strings"
)

var InvalidHead error = errors.New("Invalid HEAD")

func GetHeadBranch(repo *libgit.Repository) string {
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
func GetHeadId(repo *libgit.Repository) (string, error) {
	if headBranch := GetHeadBranch(repo); headBranch != "" {
		return repo.GetCommitIdOfBranch(GetHeadBranch(repo))
	}
	return "", InvalidHead
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
func Add(repo *libgit.Repository, args []string) {
	//	gindex, _ := ReadIndex(repo)
	for _, arg := range args {
		if _, err := os.Stat(arg); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "File %s does not exist.\n")
			continue
		} else {
			//gindex.AddFile(file)
		}
	}
	// file, err := os.Create(repo.Path + "/index-gg")
	//gindex.WriteFile(file)
}
func Branch(repo *libgit.Repository, args []string) {
	switch len(args) {
	case 0:
		branches, err := repo.GetBranches()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not get list of branches.")
			return
		}
		head := GetHeadBranch(repo)
		for _, b := range branches {
			if head == b {
				fmt.Print("* ")
			} else {
				fmt.Print("  ")
			}
			fmt.Println(b)
		}
	case 1:
		if head, err := GetHeadId(repo); err == nil {
			if cerr := libgit.CreateBranch(repo.Path, args[0], head); cerr != nil {
				fmt.Fprintf(os.Stderr, "Could not create branch: %s\n", cerr.Error())
			}
		} else {
			fmt.Fprintf(os.Stderr, "Could not create branch: %s\n", err.Error())
		}
	default:
		fmt.Fprintln(os.Stderr, "Usage: ggit branch [branchname]")
	}

}
func main() {
	repo, err := libgit.OpenRepository(".git")
	if err != nil {
		panic("Couldn't open git repository.")
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "branch":
			Branch(repo, os.Args[2:])
		case "checkout":
			Checkout(repo, os.Args[2:])
		case "add":
			Add(repo, os.Args[2:])
		}
	}
}
