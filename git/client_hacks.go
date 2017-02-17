package git

// This file contains temporary hacks that still use libgit, because
// they haven't been rewritten yet in a way that doesn't depend on it.
// They should be removed/rewritten.
import (
	libgit "github.com/driusan/git"
)

// Gets the Commit of the current HEAD as a string.
func (c *Client) GetHeadID() (string, error) {
	// Temporary hack until libgit is removed.
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return "", err
	}

	if headBranch := c.GetHeadBranch(); headBranch != "" {
		return repo.GetCommitIdOfBranch(headBranch.BranchName())
	}
	return "", InvalidHead
}
