package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// A file represents a file (or directory) relative to os.Getwd()
type File string

func (f File) Exists() bool {
	return true
}

func (f File) String() string {
	return string(f)
}

// GitDir is the .git directory of the current process.
type GitDir File

func (g GitDir) String() string {
	return string(g)
}

// WorkDir is the top level of the work directory of the current process, or
// the empty string if the --bare option is provided
type WorkDir File

type Client struct {
	GitDir  GitDir
	WorkDir WorkDir
}

func NewClient(gitDir, workDir string) (*Client, error) {
	gitdir := File(gitDir)
	if gitdir == "" {
		// TODO: Check the GIT_DIR os environment, then walk the tree
		// to find the nearest .git directory
	}
	if gitdir == "" || !gitdir.Exists() {
		return nil, fmt.Errorf("fatal: Not a git repository (or any parent)")
	}

	workdir := File(workDir)
	if workdir == "" {
		// TODO: Check the GIT_WORK_TREE os environment, then strip .git
		// from the gitdir if it doesn't exist.
	}
	return &Client{GitDir(gitdir), WorkDir(workdir)}, nil
}

func (c *Client) GetHeadID() (string, error) {
	panic("Not yet reimplemented")
	/*
		if headBranch := getHeadBranch(repo); headBranch != "" {
			return repo.GetCommitIdOfBranch(getHeadBranch(repo))
		}
		return "", InvalidHead
	*/
}

func (c *Client) GetHeadSha1() (Sha1, error) {
	panic("Not yet reimplemented")
	/*
		if headBranch := getHeadBranch(repo); headBranch != "" {
			return repo.GetCommitIdOfBranch(getHeadBranch(repo))
		}
		return "", InvalidHead
	*/
}

func (c *Client) GetBranches() ([]string, error) {
	panic("Not implemented")
}

func (c *Client) GetHeadBranch() string {
	file, _ := c.GitDir.Open("HEAD")
	value, _ := ioutil.ReadAll(file)
	if prefix := string(value[0:5]); prefix != "ref: " {
		panic("Could not understand HEAD pointer.")
	} else {
		ref := strings.Split(string(value[5:]), "/")
		if len(ref) != 3 {
			panic("Could not parse branch out of HEAD")
		}
		if ref[0] != "refs" || ref[1] != "heads" {
			panic("Unknown HEAD reference")
		}
		return strings.TrimSpace(ref[2])
	}
	return ""

}

func (c *Client) CreateBranch(name string, sha1 Sha1) error {
	panic("Unimplemented")
}

// Opens a file relative to GitDir. There should not be
// a leading slash.
func (gd GitDir) Open(f File) (*os.File, error) {
	return os.Open(gd.String() + "/" + f.String())
}
