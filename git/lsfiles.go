package git

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Finds things that aren't tracked, and creates fake IndexEntrys for them to be merged into
// the output if --others is passed.
func findUntrackedFilesFromDir(c *Client, root, parent, dir File, tracked map[IndexPath]bool, recursedir bool) (untracked []*IndexEntry) {
	files, err := ioutil.ReadDir(dir.String())
	if err != nil {
		return nil
	}
	for _, fi := range files {
		fname := File(fi.Name())
		if fi.IsDir() {
			if fi.Name() == ".git" {
				continue
			}
			if !recursedir {
				// This isn't very efficient, but let's us implement git ls-files --directory
				// without too many changes.
				indexPath, err := (parent + "/" + fname).IndexPath(c)
				if err != nil {
					panic(err)
				}
				dirHasTracked := false
				for path := range tracked {
					if strings.HasPrefix(path.String(), indexPath.String()) {
						dirHasTracked = true
						break
					}
				}
				if !dirHasTracked {
					untracked = append(untracked, &IndexEntry{PathName: indexPath})
					continue
				}
			}
			var newparent, newdir File
			if parent == "" {
				newparent = fname
			} else {
				newparent = parent + "/" + fname
			}
			if dir == "" {
				newdir = fname
			} else {
				newdir = dir + "/" + fname
			}

			recurseFiles := findUntrackedFilesFromDir(c, root, newparent, newdir, tracked, recursedir)
			untracked = append(untracked, recurseFiles...)
		} else {
			var filePath File
			if parent == "" {
				filePath = File(strings.TrimPrefix(fname.String(), root.String()))

			} else {
				filePath = File(strings.TrimPrefix((parent + "/" + fname).String(), root.String()))
			}
			indexPath, err := filePath.IndexPath(c)
			if err != nil {
				panic(err)
			}
			indexPath = IndexPath(filePath)

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

	// Show files which are unmerged. Implies Stage.
	Unmerged bool

	// If a directory is classified as "other", show only its name, not
	// its contents
	Directory bool

	// Exclude standard patterns (ie. .gitignore and .git/info/exclude)
	ExcludeStandard bool
}

// LsFiles implements the git ls-files command. It returns an array of files
// that match the options passed.
func LsFiles(c *Client, opt LsFilesOptions, files []File) ([]*IndexEntry, error) {
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

		if opt.Others {
			filesInIndex[entry.PathName] = true
		}

		if strings.HasPrefix(f.String(), "../") || len(files) > 0 {
			skip := true
			for _, explicit := range files {
				eAbs, err := filepath.Abs(explicit.String())
				if err != nil {
					return nil, err
				}
				fAbs, err := filepath.Abs(f.String())
				if err != nil {
					return nil, err
				}
				if fAbs == eAbs || strings.HasPrefix(fAbs, eAbs+"/") {
					skip = false
					break
				}
				if f.MatchGlob(explicit.String()) {
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
			continue
		}
		if opt.Deleted {
			if !f.Exists() {
				fs = append(fs, entry)
				continue
			}
		}

		if opt.Unmerged && entry.Stage() != Stage0 {
			fs = append(fs, entry)
			continue
		}

		if opt.Modified {
			_, err := f.Stat()
			// The file being deleted means it was modified
			if os.IsNotExist(err) {
				fs = append(fs, entry)
				continue
			} else if err != nil {
				return nil, err
			}
			// Checking mtime doesn't seem to be reliable, so for now we always hash
			// the file to check if it's been modified.
			//mtime, err := f.MTime()
			//if err != nil {
			//		return nil, err
			//	}

			//if size := stat.Size(); size != int64(entry.Fsize) || mtime != entry.Mtime {
			// We've done everything we can to avoid hashing the file, but now
			// we need to to avoid the case where someone changes a file, then
			// changes it back to the original contents
			hash, _, err := HashFile("blob", f.String())
			if err != nil {
				return nil, err
			}
			if hash != entry.Sha1 {
				fs = append(fs, entry)
			}
			//}
		}
	}

	if opt.Others {
		wd := File(c.WorkDir)
		others := findUntrackedFilesFromDir(c, wd+"/", wd, wd, filesInIndex, !opt.Directory)
		for _, file := range others {
			f, err := file.PathName.FilePath(c)
			if err != nil {
				return nil, err
			}
			if opt.ExcludeStandard {
				matches, err := CheckIgnore(c, CheckIgnoreOptions{NoIndex: true}, []File{f})
				if err != nil {
					return nil, err
				}
				if len(matches) == 1 && matches[0].Pattern != "" {
					continue
				}
			}
			if strings.HasPrefix(f.String(), "../") || len(files) > 0 {
				skip := true
				for _, explicit := range files {
					eAbs, err := filepath.Abs(explicit.String())
					if err != nil {
						return nil, err
					}
					fAbs, err := filepath.Abs(f.String())
					if err != nil {
						return nil, err
					}
					if fAbs == eAbs || strings.HasPrefix(fAbs, eAbs+"/") {
						skip = false
						break
					}
				}
				if skip {
					continue
				}
			}
			fs = append(fs, file)
		}
	}

	return fs, nil
}
