package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/driusan/go-git/git"
)

type stagedFile struct {
	Filename git.IndexPath
	New      bool
	Removed  bool
}

type unmergedFile struct {
	Stage1, Stage2, Stage3 git.Sha1
}

func findUntrackedFilesFromDir(c *git.Client, root, parent, dir string, tracked map[git.IndexPath]bool) (untracked []git.File) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, fi := range files {
		if fi.IsDir() {
			if fi.Name() == ".git" {
				continue
			}
			recurseFiles := findUntrackedFilesFromDir(c, root, parent+"/"+fi.Name(), dir+"/"+fi.Name(), tracked)
			untracked = append(untracked, recurseFiles...)
		} else {
			indexPath := git.IndexPath(strings.TrimPrefix(parent+"/"+fi.Name(), root))
			if _, ok := tracked[indexPath]; !ok {
				rel, err := indexPath.FilePath(c)
				if err != nil {
					panic(err)
				}
				untracked = append(untracked, rel)
			}
		}
	}
	return
}
func findUntrackedFiles(c *git.Client, tracked map[git.IndexPath]bool) []git.File {
	if c.WorkDir == "" {
		return nil
	}
	wd := string(c.WorkDir)
	return findUntrackedFilesFromDir(c, wd+"/", wd, wd, tracked)
}

// The standard git "status" command doesn't provide any kind of --prefix, so
// this does the work of status, and adds a --prefix for commit to share the
// same code as Status. Status() just parses command line options and calls
// this.
func getStatus(c *git.Client, prefix string) (string, error) {

	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
	fileInIndex := make(map[git.IndexPath]bool)
	stagedFiles := make([]stagedFile, 0)
	unstagedFiles := make([]stagedFile, 0)
	unmergedFiles := make(map[git.IndexPath]*unmergedFile)

	headFiles := make(map[git.IndexPath]git.Sha1)
	// This isn't very efficiently implemented, but it works(ish).
	head, err := git.ExpandTreeIntoIndexesById(c, "HEAD", true, false)
	if err != nil {
		return "", err
	}
	for _, head := range head {
		headFiles[head.PathName] = head.Sha1
	}

	for _, file := range idx.Objects {
		fileInIndex[file.PathName] = true
		idxsha1 := file.Sha1

		switch file.Stage() {
		case git.Stage0:
			break
		case git.Stage1:
			if uf, ok := unmergedFiles[file.PathName]; ok {
				uf.Stage1 = file.Sha1
			} else {
				unmergedFiles[file.PathName] = &unmergedFile{
					Stage1: file.Sha1,
				}
			}
			continue
		case git.Stage2:
			if uf, ok := unmergedFiles[file.PathName]; ok {
				uf.Stage2 = file.Sha1
			} else {
				unmergedFiles[file.PathName] = &unmergedFile{
					Stage2: file.Sha1,
				}
			}
			continue
		case git.Stage3:
			if uf, ok := unmergedFiles[file.PathName]; ok {
				uf.Stage3 = file.Sha1
			} else {
				unmergedFiles[file.PathName] = &unmergedFile{
					Stage3: file.Sha1,
				}
			}
			continue
		default:
			panic("Invalid stage")
		}
		relname, err := file.PathName.FilePath(c)
		if err != nil {
			panic(err)
		}
		fssha1, _, err := git.HashFile("blob", relname.String())
		if err != nil {
			if os.IsNotExist(err) {
				fssha1 = git.Sha1{}
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
			_, err := os.Stat(relname.String())
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
			_, err := os.Stat(relname.String())
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
			file, err := f.Filename.FilePath(c)
			if err != nil {
				panic(err)
			}
			if f.New {
				msg += fmt.Sprintf("%s\tnew file:\t%s\n", prefix, file)
			} else if f.Removed {
				msg += fmt.Sprintf("%s\tdeleted:\t%s\n", prefix, file)
			} else {
				msg += fmt.Sprintf("%s\tmodified:\t%s\n", prefix, file)
			}
		}
	}

	if len(unmergedFiles) != 0 {
		msg += fmt.Sprintf("%s\n", prefix)
		msg += fmt.Sprintf("%sUnmerged paths:\n", prefix)
		msg += fmt.Sprintf("%s (use \"git reset HEAD <file>...\" to unstage)\n", prefix)
		msg += fmt.Sprintf("%s (use \"git add/rm <file>...\" as appropriate to mark resolution)\n", prefix)
		msg += fmt.Sprintf("%s\n", prefix)

		for path, status := range unmergedFiles {
			file, err := path.FilePath(c)
			if err != nil {
				panic(err)
			}

			var empty git.Sha1
			if status.Stage2 == empty && status.Stage3 != empty {
				msg += fmt.Sprintf("%s\tdeleted by us:\t%s\n", prefix, file)
			} else if status.Stage2 != empty && status.Stage3 == empty {
				msg += fmt.Sprintf("%s\tdeleted by them:\t%s\n", prefix, file)
			} else if status.Stage2 == empty && status.Stage3 == empty {
				msg += fmt.Sprintf("%s\tdeleted by both:\t%s\n", prefix, file)
			} else if status.Stage2 != status.Stage3 {
				msg += fmt.Sprintf("%s\tmodified by both:\t%s\n", prefix, file)
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
			file, err := f.Filename.FilePath(c)
			if err != nil {
				panic(err)
			}

			if f.Removed {
				msg += fmt.Sprintf("%s\tdeleted:\t%s\n", prefix, file)
			} else {
				msg += fmt.Sprintf("%s\tmodified:\t%s\n", prefix, file)
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
func Status(c *git.Client, args []string) error {
	s, err := getStatus(c, "")
	if err != nil {
		return err
	}
	fmt.Printf("%s", s)
	return nil
}
