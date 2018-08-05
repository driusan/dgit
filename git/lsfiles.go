package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
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
		if fi.Name() == ".git" {
			continue
		}
		if fi.IsDir() {
			if !recursedir {
				// This isn't very efficient, but lets us implement git ls-files --directory
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

	// Do not show empty directories with --others
	NoEmptyDirectory bool

	// Exclude standard patterns (ie. .gitignore and .git/info/exclude)
	ExcludeStandard bool

	// Exclude using the provided patterns
	ExcludePatterns []string

	// Exclude using the provided file with the patterns
	ExcludeFiles []File

	// Exclude using additional patterns from each directory
	ExcludePerDirectory []File

	ErrorUnmatch bool
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
	if opt.Others || opt.ErrorUnmatch {
		filesInIndex = make(map[IndexPath]bool)
	}

	for _, entry := range index.Objects {
		f, err := entry.PathName.FilePath(c)
		if err != nil {
			return nil, err
		}

		if opt.Others || opt.ErrorUnmatch {
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
			if f.IsDir() {
				fs = append(fs, entry)
				continue
			}
			_, err := f.Stat()
			// The file being deleted means it was modified
			if os.IsNotExist(err) {
				fs = append(fs, entry)
				continue
			} else if err != nil {
				return nil, err
			}

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
		}
	}

	if opt.ErrorUnmatch {
		for _, file := range files {
			indexPath, err := file.IndexPath(c)
			if err != nil {
				return nil, err
			}
			if _, ok := filesInIndex[indexPath]; !ok {
				fmt.Printf("%v", filesInIndex)
				return nil, fmt.Errorf("error: pathspec '%v' did not match any file(s) known to git", file)
			}
		}
	}

	if opt.Others {
		wd := File(c.WorkDir)
		others := findUntrackedFilesFromDir(c, wd+"/", wd, wd, filesInIndex, !opt.Directory)
		otherFiles := make([]File, 0, len(others))

		for _, file := range others {
			f, err := file.PathName.FilePath(c)
			if err != nil {
				return nil, err
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

			if f.IsDir() && opt.Directory {
				if opt.NoEmptyDirectory {
					if files, err := ioutil.ReadDir(f.String()); len(files) == 0 && err == nil {
						continue
					}
				}
				f += "/"
			}

			otherFiles = append(otherFiles, f)
		}

		ignorePatterns := []IgnorePattern{}

		if opt.ExcludeStandard {
			standardPatterns, err := StandardIgnorePatterns(c, otherFiles)
			if err != nil {
				return nil, err
			}
			ignorePatterns = append(ignorePatterns, standardPatterns...)
		}
		for _, pattern := range opt.ExcludePatterns {
			ignorePatterns = append(ignorePatterns, IgnorePattern{Pattern: pattern, Source: "", LineNum: 1, Scope: ""})
		}

		for _, file := range opt.ExcludeFiles {
			patterns, err := ParseIgnorePatterns(c, file, "")
			if err != nil {
				return nil, err
			}
			ignorePatterns = append(ignorePatterns, patterns...)
		}

		matches, err := MatchIgnores(c, ignorePatterns, otherFiles)
		if err != nil {
			return nil, err
		}

		for _, match := range matches {
			if match.Pattern == "" { // TODO add ignore here
				indexPath, err := match.PathName.IndexPath(c)
				if err != nil {
					return nil, err
				}
				// Add a "/" if --directory is set so that it sorts properly in some
				// edge cases.
				if match.PathName.IsDir() && opt.Directory {
					indexPath += "/"

				}
				fs = append(fs, &IndexEntry{PathName: indexPath})
			}
		}
	}

	sort.Sort(ByPath(fs))
	return fs, nil
}
