package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
)

// A HashDiff represents a single line in a git diff-index type output.
type HashDiff struct {
	Name             IndexPath
	Src, Dst         TreeEntry
	SrcSize, DstSize uint
}

func (h HashDiff) String() string {
	var status string = "?"

	empty := Sha1{}
	if h.Src.Sha1 == empty && h.Dst.Sha1 != empty {
		status = "A"
	} else if h.Src.Sha1 != empty && h.Dst.Sha1 == empty {
		if h.Dst.FileMode == 0 {
			status = "D"
		} else {
			status = "M"
		}
	} else {
		status = "M"
	}
	return fmt.Sprintf(":%0.6o %0.6o %v %v %v	%v", h.Src.FileMode, h.Dst.FileMode, h.Src.Sha1, h.Dst.Sha1, status, h.Name)
}

// Returns a diff in the format of the command "diff". Note: this invokes
// an external diff tool. It should be rewritten in Go to avoid the overhead
// (and the possibility that diff isn't installed.)
func (h HashDiff) ExternalDiff(c *Client, s1, s2 TreeEntry, f File, opts DiffCommonOptions) (string, error) {
	tmpfile1, err := ioutil.TempFile("", "gitdiff")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpfile1.Name())

	var emptySha Sha1
	if s1.Sha1 != emptySha {
		obj, err := c.GetObject(s1.Sha1)
		if err != nil {
			return "", err
		}
		tmpfile1.Write(obj.GetContent())
	}

	tmpfile1.Close()

	tmpfile2, err := ioutil.TempFile("", "gitdiff")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpfile2.Name())

	var file2 = tmpfile2.Name()
	if s2.Sha1 != emptySha {
		obj, err := c.GetObject(s2.Sha1)
		if err != nil {
			return "", err
		}
		tmpfile2.Write(obj.GetContent())
	} else if s2.FileMode != 0 {
		file2 = f.String()
	}
	tmpfile2.Close()

	indexPath, err := f.IndexPath(c)
	if err != nil {
		// If it couldn't be converted, fall back on the file name.
		indexPath = IndexPath(f)
	}
	diffcmd := exec.Command(posixDiff, "-u", "-U", strconv.Itoa(opts.NumContextLines), "-L", ("a/" + indexPath).String(), "-L", ("b/" + indexPath).String(), tmpfile1.Name(), file2)
	// diff returns an error code if there's any differences, so just throw
	// away the error.
	diffcmd.Stderr = os.Stderr
	out, _ := diffcmd.Output()
	return string(out), nil

}

// Implement the sort interface on *GitIndexEntry, so that
// it's easy to sort by name.
type ByName []HashDiff

func (g ByName) Len() int           { return len(g) }
func (g ByName) Swap(i, j int)      { g[i], g[j] = g[j], g[i] }
func (g ByName) Less(i, j int) bool { return g[i].Name < g[j].Name }
