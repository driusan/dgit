package git

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// A file represents a file (or directory) relative to os.Getwd()
type File string

// Determines if the file exists on the filesystem.
func (f File) Exists() bool {
	if _, err := os.Lstat(string(f)); os.IsNotExist(err) {
		return false
	}
	return true
}

func (f File) String() string {
	return string(f)
}

// Appends the value val to the end of the file f.
// Not that f must already exist.
func (f File) Append(val string) error {
	if !f.Exists() {
		return fmt.Errorf("File %s does not exist", f)
	}
	fi, err := os.OpenFile(f.String(), os.O_RDWR|os.O_APPEND, 0660)
	if err != nil {
		return err
	}
	defer fi.Close()
	fmt.Fprintf(fi, "%s", val)
	return nil
}

// Normalizes the file name that's relative to the current working directory
// to be relative to the workdir root. Ie. convert it from a file system
// path to an index path.
func (f File) IndexPath(c *Client) (IndexPath, error) {
	p, err := filepath.Abs(f.String())
	if err != nil {
		return "", err
	}
	// BUG(driusan): This should verify that there is a prefix and return
	// an error if it's outside of the tree.
	return IndexPath(strings.TrimPrefix(p, string(c.WorkDir)+"/")), nil
}

// Returns stat information for the given file.
func (f File) Stat() (os.FileInfo, error) {
	return os.Stat(string(f))
}

// Returns lstat information for the given file.
func (f File) Lstat() (os.FileInfo, error) {
	return os.Lstat(string(f))
}

func (f File) IsDir() bool {
	stat, err := f.Lstat()
	if err != nil {
		// If we couldn't stat it, it's not a directory..
		return false
	}
	return stat.IsDir()

}

// Returns true if the file is inside a submodule and the submodule file.
// FIXME: invert this when submodules are implemented
func (f File) IsInSubmodule(c *Client) (bool, File, error) {
	abs, err := filepath.Abs(f.String())
	if err != nil {
		return false, File(""), err
	}

	for abs != c.WorkDir.String() {
		stat, _ := os.Lstat(filepath.Join(abs, ".git"))
		if stat != nil {
			submodule, err := File(abs).IndexPath(c)
			if err != nil {
				return false, File(""), err
			}
			return true, File(string(submodule)), nil
		}

		abs = filepath.Dir(abs)
	}

	return false, File(""), nil
}

func (f File) IsInsideSymlink() (bool, error) {
	abs, err := filepath.Abs(f.String())
	if err != nil {
		return false, err
	}

	dir := filepath.Dir(abs)

	evalPath, _ := filepath.EvalSymlinks(dir)
	if evalPath != "" && evalPath != dir {
		return true, nil
	}

	return false, nil
}

func (f File) IsSymlink() bool {
	stat, err := f.Lstat()
	if err != nil {
		// If we couldn't stat it, it's not a directory..
		return false
	}
	// This is probably not robust. It's assuming every OS
	// uses the same modes as git, but is probably good enough.
	return stat.Mode()&os.ModeSymlink == os.ModeSymlink
}

func (f File) Create() error {
	dir := File(filepath.Dir(f.String()))
	if !dir.Exists() {
		if err := os.MkdirAll(dir.String(), 0755); err != nil {
			return err
		}
	}

	fi, err := os.Create(f.String())
	fi.Close()
	return err
}

// Reads the entire contents of file and return as a string. Note
// that this should only be used for very small files (like refspecs)
//
func (f File) ReadAll() (string, error) {
	val, err := ioutil.ReadFile(f.String())
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// Reads the first line of File. (This is primarily to extract commit message
// lines for reflogs)
func (f File) ReadFirstLine() (string, error) {
	if !f.Exists() {
		return "", fmt.Errorf("File %s does not exist", f)
	}
	fi, err := os.Open(f.String())
	if err != nil {
		return "", err
	}
	defer fi.Close()
	scanner := bufio.NewScanner(fi)
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return scanner.Text(), nil
}

func (f File) Remove() error {
	return os.Remove(f.String())
}

func (f File) Open() (*os.File, error) {
	return os.Open(f.String())
}

// Returns true if f matches the filesystem glob pattern pattern.
func (f File) MatchGlob(pattern string) bool {
	m, err := filepath.Match(pattern, string(f))
	if err != nil {
		panic(err)
	}
	return m
}
