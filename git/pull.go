package git

import ()

type PullOptions struct {
	FetchOptions
	MergeOptions
}

func Pull(c *Client, opts PullOptions, repository string, remotebranches []string) error {
	err := Fetch(c, opts.FetchOptions, repository)
	if err != nil {
		return err
	}

	others := make([]Commitish, 0, len(remotebranches))
	for _, name := range remotebranches {
		c, err := RevParseCommitish(c, &RevParseOptions{}, name)
		if err != nil {
			return err
		}
		others = append(others, c)
	}

	return Merge(c, opts.MergeOptions, others)
}
