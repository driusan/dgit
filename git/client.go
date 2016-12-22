package git

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/driusan/go-git/zlib"
)

// A file represents a file (or directory) relative to os.Getwd()
type File string

// An IndexPath represents a file in the index. ie. a File path relative
// to the Git WorkDir, not the current working directory.
type IndexPath string

func (f IndexPath) String() string {
	return string(f)
}

func (f File) Exists() bool {
	if _, err := os.Stat(string(f)); os.IsNotExist(err) {
		return false
	}
	return true
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

	return IndexPath(strings.TrimPrefix(p, string(c.WorkDir)+"/")), nil
}

func (f File) Stat() (os.FileInfo, error) {
	return os.Stat(f.String())
}

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
func (c *Client) ResetWorkTree() error {
	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		return err
	}
	for _, indexEntry := range idx.Objects {
		obj, err := c.GetObject(indexEntry.Sha1)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not retrieve %x for %s: %s\n", indexEntry.Sha1, indexEntry.PathName, err)
			continue
		}
		if strings.Index(indexEntry.PathName, "/") > 0 {
			os.MkdirAll(filepath.Dir(indexEntry.PathName), 0755)
		}
		err = ioutil.WriteFile(indexEntry.PathName, obj.GetContent(), os.FileMode(indexEntry.Mode))
		if err != nil {
			continue
		}
		os.Chmod(indexEntry.PathName, os.FileMode(indexEntry.Mode))
	}
	return nil
}

func (c *Client) GetSymbolicRefCommit(r RefSpec) (CommitID, error) {
	file, err := c.GitDir.Open(File(r))
	if err != nil {
		return CommitID{}, err
	}
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return CommitID{}, err
	}
	sha, err := Sha1FromString(string(data))
	return CommitID(sha), err
}
func (c *Client) GetBranchCommit(b string) (CommitID, error) {
	file, err := c.GitDir.Open(File("refs/heads/" + b))
	if err != nil {
		return CommitID{}, err
	}
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return CommitID{}, err
	}
	sha, err := Sha1FromString(string(data))
	return CommitID(sha), err
}

func (c *Client) GetBranches() (branches []string, err error) {
	files, err := ioutil.ReadDir(c.GitDir.String() + "/refs/heads")
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		branches = append(branches, f.Name())
	}
	return
}

func (c *Client) CreateBranch(name string, commit Commitish) error {
	id, err := commit.CommitID(c)
	if err != nil {
		return err
	}

	return c.GitDir.WriteFile(File("refs/heads/"+name), []byte(id.String()), 0644)
}

func (c *Client) GetAuthor() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("home") // On some OSes, it is home
	}
	configFile, err := os.Open(home + "/.gitconfig")
	config := ParseConfig(configFile)
	if err != nil {
		panic(err)
	}

	name := config.GetConfig("user.name")
	email := config.GetConfig("user.email")
	return fmt.Sprintf("%s <%s>", name, email)
}

// Resets the index to the Treeish tree and save the results in
// the file named indexname
func (c *Client) ResetIndex(tree Treeish, indexname string) error {
	// If the index doesn't exist, idx is a new index, so ignore
	// the path error that ReadIndex is returning
	idx, _ := c.GitDir.ReadIndex()
	idx.ResetIndex(c, tree)

	f, err := c.GitDir.Create(File(indexname))
	if err != nil {
		return err
	}
	defer f.Close()
	return idx.WriteIndex(f)
}

func (c *Client) WriteObject(objType string, rawdata []byte) (Sha1, error) {
	obj := []byte(fmt.Sprintf("%s %d\000", objType, len(rawdata)))
	obj = append(obj, rawdata...)
	sha := sha1.Sum(obj)

	if have, _, err := c.HaveObject(fmt.Sprintf("%x", sha)); have == true || err != nil {
		if err != nil {
			return Sha1{}, err

		}

		return Sha1(sha), ObjectExists

	}
	directory := fmt.Sprintf("%x", sha[0:1])
	file := fmt.Sprintf("%x", sha[1:])

	os.MkdirAll(c.GitDir.String()+"/objects/"+directory, os.FileMode(0755))
	f, err := c.GitDir.Create(File("objects/" + directory + "/" + file))
	if err != nil {
		return Sha1{}, err
	}
	defer f.Close()
	w := zlib.NewWriter(f)
	if _, err := w.Write(obj); err != nil {
		return Sha1{}, err
	}
	defer w.Close()
	return Sha1(sha), nil
}
