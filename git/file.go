package git

import (
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
	if _, err := os.Stat(string(f)); os.IsNotExist(err) {
		return false
	}
	return true
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
	fmt.Fprintf(fi, "%s", val)
	return nil
}

func (f File) String() string {
	return string(f)
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
	return os.Stat(f.String())
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
