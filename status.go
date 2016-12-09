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

// The standard git "status" command doesn't provide any kind of --prefix, so
// this does the work of status, and adds a --prefix for commit to share the
// same code as Status. Status() just parses command line options and calls
// this.
func getStatus(c *Client, repo *libgit.Repository, prefix string) (string, error) {
	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
	fileInIndex := make(map[string]bool)
	stagedFiles := make([]stagedFile, 0)
	unstagedFiles := make([]stagedFile, 0)

	headFiles := make(map[string]Sha1)
	// This isn't very efficiently implemented, but it works(ish).
	head, err := expandTreeIntoIndexesById(c, repo, "HEAD", true, false)
	for _, head := range head {
		headFiles[head.PathName] = head.Sha1
	}

	for _, file := range idx.Objects {
		fileInIndex[file.PathName] = true
		idxsha1 := file.Sha1

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
		if fssha1 != idxsha1 {
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
		if headSha1 != idxsha1 {
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

	var msg string

	untracked := findUntrackedFiles(c, fileInIndex)
	if len(stagedFiles) != 0 {
		msg += fmt.Sprintf("%sChanges to be committed:\n", prefix)
		msg += fmt.Sprintf("%s (use \"git reset HEAD <file>...\" to unstage)\n", prefix)
		msg += fmt.Sprintf("%s\n", prefix)
		for _, f := range stagedFiles {
			if f.New {
				msg += fmt.Sprintf("%s\tnew file:\t%s\n", prefix, f.Filename)
			} else if f.Removed {
				msg += fmt.Sprintf("%s\tdeleted:\t%s\n", prefix, f.Filename)
			} else {
				msg += fmt.Sprintf("%s\tmodified:\t%s\n", prefix, f.Filename)
			}
		}
	}
	if len(unstagedFiles) != 0 {
		msg += fmt.Sprintf("%s\n", prefix)
		msg += fmt.Sprintf("%sChanges not staged for commit:\n", prefix)
		msg += fmt.Sprintf("%s\n", prefix)
		msg += fmt.Sprintf("%s  (use \"git add <file>...\" to update what will be committed)\n", prefix)
		msg += fmt.Sprintf("%s  (use \"git checkout -- <file>...\" to discard changes in working directory)\n", prefix)
		msg += fmt.Sprintf("%s\n", prefix)
		for _, f := range unstagedFiles {
			if f.Removed {
				msg += fmt.Sprintf("\tdeleted:\t%s\n", f.Filename)
			} else {
				msg += fmt.Sprintf("\tmodified:\t%s\n", f.Filename)
			}
		}
	}

	if len(untracked) != 0 {
		msg += fmt.Sprintf("%s\n", prefix)
		msg += fmt.Sprintf("%sUntracked files:\n", prefix)
		msg += fmt.Sprintf("%s  (use \"git add <file>...\" to include in what will be committed)\n", prefix)
		msg += fmt.Sprintf("%s\n", prefix)
		for _, f := range untracked {
			msg += fmt.Sprintf("%s\t%s\n", prefix, f)
		}
	}

	if len(unstagedFiles) == 0 && len(stagedFiles) == 0 && len(untracked) == 0 {
		return "", fmt.Errorf("nothing to commit, working tree clean")
	}
	return msg, nil
}
func Status(c *Client, repo *libgit.Repository, args []string) error {
	s, err := getStatus(c, repo, "")
	if err != nil {
		return err
	}
	fmt.Printf("%s", s)
	return nil
}
