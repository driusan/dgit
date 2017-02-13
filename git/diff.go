package git

// Describes the options that may be specified on the command line for
// "git diff".
type DiffOptions struct {
	DiffCommonOptions

	Staged bool
}

// DiffFiles implements the git diff-files command.
// It compares the file system to the index.
func Diff(c *Client, opt DiffOptions, paths []string) ([]HashDiff, error) {
	if opt.Staged {
		head, err := c.GetHeadCommit()
		if err != nil {
			return nil, err
		}
		return DiffIndex(c,
			DiffIndexOptions{
				DiffCommonOptions: opt.DiffCommonOptions,
				Cached:            true,
			},
			head,
			paths)
	}
	return DiffFiles(c,
		DiffFilesOptions{
			DiffCommonOptions: opt.DiffCommonOptions,
		},
		paths)
}
