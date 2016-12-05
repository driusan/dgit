package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	libgit "github.com/driusan/git"
)

// Gets a list of all filenames in the working tree
func getWorkTreeFiles() []string {
	return nil
}

type stagedFile struct {
	Filename string
	New      bool
	Removed  bool
}

func findUntrackedFilesFromDir(root, parent, dir string, tracked map[string]bool) (untracked []string) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, fi := range files {
		if fi.IsDir() {
			if fi.Name() == ".git" {
				continue
			}
			recurseFiles := findUntrackedFilesFromDir(root, parent+"/"+fi.Name(), dir+"/"+fi.Name(), tracked)
			untracked = append(untracked, recurseFiles...)
		} else {
			relFile := strings.TrimPrefix(parent+"/"+fi.Name(), root)
			if _, ok := tracked[relFile]; !ok {
				untracked = append(untracked, relFile)
			}
		}
	}
	return

}
func findUntrackedFiles(c *Client, tracked map[string]bool) []string {
	if c.WorkDir == "" {
		return nil
	}
	wd := string(c.WorkDir)
	return findUntrackedFilesFromDir(wd+"/", wd, wd, tracked)
}

func Status(c *Client, repo *libgit.Repository, args []string) {
	idx, err := ReadIndex(c, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
	fileInIndex := make(map[string]bool)
	stagedFiles := make([]stagedFile, 0)
	unstagedFiles := make([]stagedFile, 0)

	headFiles := make(map[string]Sha1)
	// This isn't very efficiently implemented, but it works(ish).
	head, err := expandTreeIntoIndexesById(c, repo, "HEAD")
	for _, head := range head {
		headFiles[head.PathName] = Sha1(head.Sha1[:])
	}

	for _, file := range idx.Objects {
		fileInIndex[file.PathName] = true
		idxsha1 := fmt.Sprintf("%x", file.Sha1)

		fssha1, err := HashFile("blob", file.PathName)
		if err != nil {
			if os.IsNotExist(err) {
				fssha1 = Sha1{}
			} else {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				continue
			}
		}

		headSha1, headExists := headFiles[file.PathName]
		if !headExists {
			stagedFiles = append(stagedFiles,
				stagedFile{
					Filename: file.PathName,
					New:      true,
					Removed:  false,
				},
			)
			continue
		}
		if fssha1.String() != idxsha1 {
			_, err := os.Stat(file.PathName)
			if os.IsNotExist(err) {
				unstagedFiles = append(unstagedFiles,
					stagedFile{file.PathName, false, true},
				)
			} else {
				unstagedFiles = append(unstagedFiles,
					stagedFile{file.PathName, false, false},
				)
			}

		}
		if headSha1.String() != idxsha1 {
			_, err := os.Stat(file.PathName)
			if os.IsNotExist(err) {
				stagedFiles = append(stagedFiles,
					stagedFile{file.PathName, false, true},
				)
			} else {
				stagedFiles = append(stagedFiles,
					stagedFile{file.PathName, false, false},
				)
			}
		}
	}

	for file, _ := range headFiles {
		if _, ok := fileInIndex[file]; !ok {
			stagedFiles = append(stagedFiles,
				stagedFile{file, false, true},
			)
		}
	}
	untracked := findUntrackedFiles(c, fileInIndex)
	if len(stagedFiles) != 0 {
		fmt.Printf(
			`Changes to be committed:
  (use "git reset HEAD <file>..." to unstage)

`)
		for _, f := range stagedFiles {
			if f.New {
				fmt.Printf("\tnew file:\t%s\n", f.Filename)
			} else if f.Removed {
				fmt.Printf("\tdeleted:\t%s\n", f.Filename)
			} else {
				fmt.Printf("\tmodified:\t%s\n", f.Filename)
			}
		}
	}
	if len(unstagedFiles) != 0 {
		fmt.Printf(
			`
Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git checkout -- <file>..." to discard changes in working directory)

`)
		for _, f := range unstagedFiles {
			if f.Removed {
				fmt.Printf("\tdeleted:\t%s\n", f.Filename)
			} else {
				fmt.Printf("\tmodified:\t%s\n", f.Filename)
			}
		}
	}

	if len(untracked) != 0 {
		fmt.Printf(
			`
Untracked files:
  (use "git add <file>..." to include in what will be committed)

`)
		for _, f := range untracked {
			fmt.Printf("\t%s\n", f)
		}
	}

	if len(unstagedFiles) == 0 && len(stagedFiles) == 0 && len(untracked) == 0 {
		fmt.Println("nothing to commit, working tree clean")
	}
	return
}
