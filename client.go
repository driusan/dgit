package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	libgit "github.com/driusan/git"
)

// A file represents a file (or directory) relative to os.Getwd()
type File string

func (f File) Exists() bool {
	if _, err := os.Stat(string(f)); os.IsNotExist(err) {
		return false
	}
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

func (g GitDir) Exists() bool {
	return File(g).Exists()
}

// Returns a file named f, relative to GitDir
func (g GitDir) File(f File) File {
	return File(g) + "/" + f
}

func (g GitDir) WriteFile(f File, data []byte, perm os.FileMode) error {
	return ioutil.WriteFile(g.File(f).String(), data, perm)
}

// WorkDir is the top level of the work directory of the current process, or
// the empty string if the --bare option is provided
type WorkDir File

type Client struct {
	GitDir  GitDir
	WorkDir WorkDir
}

// Walks from the current directory to find a .git directory
func findGitDir() GitDir {
	startPath, err := os.Getwd()
	if err != nil {
		return ""
	}
	if dirinfo, err := os.Stat(startPath + "/.git"); err == nil && dirinfo.IsDir() {
		return GitDir(startPath) + "/.git"
	}
	pieces := strings.Split(startPath, "/")

	for i := len(pieces); i > 0; i -= 1 {
		dir := strings.Join(pieces[0:i], "/")
		if dirinfo, err := os.Stat(dir + "/.git"); err == nil && dirinfo.IsDir() {
			return GitDir(dir) + "/.git"
		}
	}
	return ""
}

func NewClient(gitDir, workDir string) (*Client, error) {
	gitdir := GitDir(gitDir)
	if gitdir == "" {
		gitdir = GitDir(os.Getenv("GIT_DIR"))
		if gitdir == "" {
			gitdir = findGitDir()
		}
	}

	if gitdir == "" || !gitdir.Exists() {
		return nil, fmt.Errorf("fatal: Not a git repository (or any parent)")
	}

	workdir := WorkDir(workDir)
	if workdir == "" {
		workdir = WorkDir(os.Getenv("GIT_WORK_TREE"))
		if workdir == "" {
			workdir = WorkDir(strings.TrimSuffix(gitdir.String(), "/.git"))
		}
		// TODO: Check the GIT_WORK_TREE os environment, then strip .git
		// from the gitdir if it doesn't exist.
	}
	return &Client{GitDir(gitdir), WorkDir(workdir)}, nil
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

/*
func (c *Client) GetHeadSha1() (Sha1, error) {
	panic("Not yet reimplemented")
		if headBranch := getHeadBranch(repo); headBranch != "" {
			return repo.GetCommitIdOfBranch(getHeadBranch(repo))
		}
		return "", InvalidHead
}
*/

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

func (c *Client) HaveObject(idStr string) (found, packed bool, err error) {
	// As a temporary hack use libgit, because I don't have time to
	// make sure pack files are looked into properly yet.
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return false, false, err
	}
	return repo.HaveObject(idStr)
}
func (c *Client) CreateBranch(name string, sha1 Sha1) error {
	panic("Unimplemented")
}

func (c *Client) ExecEditor(f File) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		fmt.Fprintf(os.Stderr, "Warning: EDITOR environment not set. Falling back on ed...\n")
		editor = "ed"
	}
	cmd := exec.Command(editor, f.String())
	return cmd.Run()
}

// Opens a file relative to GitDir. There should not be
// a leading slash.
func (gd GitDir) Open(f File) (*os.File, error) {
	return os.Open(gd.String() + "/" + f.String())
}

// Creates a file relative to GitDir. There should not be
// a leading slash.
func (gd GitDir) Create(f File) (*os.File, error) {
	return os.Create(gd.String() + "/" + f.String())
}
