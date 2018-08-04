package git

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// Describes the options that may be specified on the command line for
// "git diff".
type DiffOptions struct {
	DiffCommonOptions

	Staged bool

	NoIndex bool
}

// DiffFiles implements the git diff-files command.
// It compares the file system to the index.
func Diff(c *Client, opt DiffOptions, paths []File) ([]HashDiff, error) {
	if opt.NoIndex {
		if len(paths) != 2 {
			return nil, fmt.Errorf("Must provide 2 paths for git diff --no-index")
		}

		// Just invoke the system diff command, we can't return a HashDiff
		// since we're not working things that are tracked by the repo.
		// we just directly invoke diff if --no-index is specified.
		diffcmd := exec.Command(posixDiff, "-U", strconv.Itoa(opt.NumContextLines), paths[0].String(), paths[1].String())
		diffcmd.Stderr = os.Stderr
		diffcmd.Stdout = os.Stdout

		if err := diffcmd.Run(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if opt.Staged {
		head, err := c.GetHeadCommit()
		if err != nil {
			return nil, err
		}
		index, _ := c.GitDir.ReadIndex()
		return DiffIndex(c,
			DiffIndexOptions{
				DiffCommonOptions: opt.DiffCommonOptions,
				Cached:            true,
			},
			index,
			head,
			paths)
	}
	return DiffFiles(c,
		DiffFilesOptions{
			DiffCommonOptions: opt.DiffCommonOptions,
		},
		paths)
}
