package git

import (
	"os"
	"os/exec"
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
		// we just directly invoke diff if --no-index is specified.
		var strpaths []string
		for _, path := range paths {
			strpaths = append(strpaths, string(path))
		}

		// Just invoke the system diff command, we can't return a HashDiff
		// since we're not working things that are tracked by the repo.
		diffcmd := exec.Command(posixDiff, strpaths...)
		diffcmd.Stderr = os.Stderr
		diffcmd.Stdout = os.Stderr

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
