package git

import (
	//"fmt"
	"io/ioutil"
	"strings"
)

// Finds things that aren't tracked, and creates fake IndexEntrys for them to be merged into
// the output if --others is passed.
func findUntrackedFilesFromDir(c *Client, root, parent, dir string, tracked map[IndexPath]bool) (untracked []*IndexEntry) {
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
			indexPath := IndexPath(strings.TrimPrefix(parent+"/"+fi.Name(), root))

			if _, ok := tracked[indexPath]; !ok {
				untracked = append(untracked, &IndexEntry{PathName: indexPath})
			}
		}
	}
	return
}

// Describes the options that may be specified on the command line for
// "git diff-index". Note that only raw mode is currently supported, even
// though all the other options are parsed/set in this struct.
type LsFilesOptions struct {
	// Types of files to show
	Cached, Deleted, Modified, Others bool

	// Invert exclusion logic
	Ignored bool

	// Show stage status instead of just file name
	Stage bool
}

// LsFiles implements the git ls-files command. It returns an array of files
// that match the options passed.
func LsFiles(c *Client, opt *LsFilesOptions, files []string) ([]*IndexEntry, error) {
	var fs []*IndexEntry
	index, err := c.GitDir.ReadIndex()
	if err != nil {
		return nil, err
	}

	// We need to keep track of what's in the index if --others is passed.
	// Keep a map instead of doing an O(n) search every time.
	var filesInIndex map[IndexPath]bool
	if opt.Others {
		filesInIndex = make(map[IndexPath]bool)

	}

	for _, entry := range index.Objects {
		f, err := entry.PathName.FilePath(c)
		if err != nil {
			return nil, err
		}

		if strings.HasPrefix(f.String(), "../") {
			skip := true
			for _, explicit := range files {
				if strings.HasPrefix(f.String(), explicit) {
					skip = false
					break
				}
			}
			if skip {
				continue
			}
		}
		if opt.Cached {
			fs = append(fs, entry)
		}
		if opt.Deleted {
			if !f.Exists() {
				fs = append(fs, entry)
			}
		}
		if opt.Modified {
			// An error can just mean it's deleted without --deleted
			// passed, so ignore the error.
			hash, _, _ := HashFile("blob", f.String())
			if hash != entry.Sha1 {
				fs = append(fs, entry)
			}
		}
		if opt.Others {
			filesInIndex[entry.PathName] = true
		}

	}

	if opt.Others {
		wd := string(c.WorkDir)
		others := findUntrackedFilesFromDir(c, wd+"/", wd, wd, filesInIndex)
		fs = append(fs, others...)
	}

	return fs, nil
}
