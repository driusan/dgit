package main

// This file contains temporary hacks that still use libgit, because
// they haven't been rewritten yet in a way that doesn't depend on it.
// They should be removed/rewritten.
import (
	libgit "github.com/driusan/git"
)

func (c *Client) GetBranches() ([]string, error) {
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return nil, err
	}
	return repo.GetBranches()
}

func (c *Client) CreateBranch(name string, sha1 Sha1) error {
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return err
	}
	return repo.CreateBranch(name, sha1.String())
}

func (c *Client) HaveObject(idStr string) (found, packed bool, err error) {
	// As a temporary hack use libgit, because I don't have time to
	// make sure pack files are looked into properly yet.
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return false, false, err
	}
	return repo.HaveObject(idStr)
}

func (c *Client) GetHeadID() (string, error) {
	// Temporary hack until libgit is removed.
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return "", err
	}
	if headBranch := c.GetHeadBranch(); headBranch != "" {
		return repo.GetCommitIdOfBranch(c.GetHeadBranch())
	}
	return "", InvalidHead

}
