package git

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Finds things that aren't tracked, and creates fake IndexEntrys for them to be merged into
// the output if --others is passed.
func findUntrackedFilesFromDir(c *Client, opts LsFilesOptions, root, parent, dir File, tracked map[IndexPath]bool, recursedir bool, ignorePatterns []IgnorePattern) (untracked []*IndexEntry) {
	files, err := ioutil.ReadDir(dir.String())
	if err != nil {
		return nil
	}
	for _, ignorefile := range opts.ExcludePerDirectory {
		ignoreInDir := ignorefile
		if dir != "" {
			ignoreInDir = dir + "/" + ignorefile
		}

		if ignoreInDir.Exists() {
			log.Println("Adding excludes from", ignoreInDir)

			patterns, err := ParseIgnorePatterns(c, ignoreInDir, dir)
			if err != nil {
				continue
			}
			ignorePatterns = append(ignorePatterns, patterns...)
		}
	}
files:
	for _, fi := range files {
		fname := File(fi.Name())
		if fi.Name() == ".git" {
			continue
		}
		for _, pattern := range ignorePatterns {
			var name File
			if parent == "" {
				name = fname
			} else {
				name = parent + "/" + fname
			}
			if pattern.Matches(name.String(), fi.IsDir()) {
				continue files
			}
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
					if opts.Directory {
						if opts.NoEmptyDirectory {
							if files, err := ioutil.ReadDir(fname.String()); len(files) == 0 && err == nil {
								continue
							}
						}
						indexPath += "/"
					}
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

			recurseFiles := findUntrackedFilesFromDir(c, opts, root, newparent, newdir, tracked, recursedir, ignorePatterns)
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

	// Show files which need to be removed for checkout-index to succeed
	Killed bool

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
		if opt.Killed {
			// We go through each parent to check if it exists on the filesystem
			// until we find a directory (which means there's no more files getting
			// in the way of os.MkdirAll from succeeding in CheckoutIndex)
			pathparent := filepath.Clean(path.Dir(f.String()))

			for pathparent != "" && pathparent != "." {
				f := File(pathparent)
				if f.IsDir() {
					// We found a directory, so there's nothing
					// getting in the way
					break
				} else if f.Exists() {
					// It's not a directory but it exists,
					// so we need to delete it
					indexPath, err := f.IndexPath(c)
					if err != nil {
						return nil, err
					}
					fs = append(fs, &IndexEntry{PathName: indexPath})
				}
				// check the next level of the directory path
				pathparent, _ = filepath.Split(filepath.Clean(pathparent))
			}
			if f.IsDir() {
				indexPath, err := f.IndexPath(c)
				if err != nil {
					return nil, err
				}
				fs = append(fs, &IndexEntry{PathName: indexPath})
			}
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
				return nil, fmt.Errorf("error: pathspec '%v' did not match any file(s) known to git", file)
			}
		}
	}

	if opt.Others {
		wd := File(c.WorkDir)

		ignorePatterns := []IgnorePattern{}

		if opt.ExcludeStandard {
			opt.ExcludeFiles = append(opt.ExcludeFiles, File(filepath.Join(c.GitDir.String(), "info/exclude")))
			opt.ExcludePerDirectory = append(opt.ExcludePerDirectory, ".gitignore")
		}

		for _, file := range opt.ExcludeFiles {
			patterns, err := ParseIgnorePatterns(c, file, "")
			if err != nil {
				return nil, err
			}
			ignorePatterns = append(ignorePatterns, patterns...)
		}

		for _, pattern := range opt.ExcludePatterns {
			ignorePatterns = append(ignorePatterns, IgnorePattern{Pattern: pattern, Source: "", LineNum: 1, Scope: ""})
		}

		others := findUntrackedFilesFromDir(c, opt, wd+"/", wd, wd, filesInIndex, !opt.Directory, ignorePatterns)
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

			fs = append(fs, file)
		}
	}

	sort.Sort(ByPath(fs))
	return fs, nil
}
