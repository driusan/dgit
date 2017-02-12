package git

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/driusan/dgit/zlib"
)

// An IndexPath represents a file in the index. ie. a File path relative
// to the Git WorkDir, not the current working directory.
type IndexPath string

func (f IndexPath) String() string {
	return string(f)
}

// Returns the IndexPath as a filename relative to Getwd, in order to convert
// from Indexes to Working directory paths.
func (f IndexPath) FilePath(c *Client) (File, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	file := c.WorkDir.String() + "/" + f.String()

	rel, err := filepath.Rel(cwd, file)
	if err != nil {
		return "", err
	}
	return File(rel), err
}

// A GitDir represents the .git/ directory of a repository. It should not
// have a trailing slash.
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

// WriteFile writes data to the file f, using permission perm for the
// file if it does not already exist.
//
// WriteFile will overwrite existing file contents.
func (g GitDir) WriteFile(f File, data []byte, perm os.FileMode) error {
	return ioutil.WriteFile(g.File(f).String(), data, perm)
}

// WorkDir is the top level of the work directory of the current process, or
// the empty string if the --bare option is provided
type WorkDir File

func (f WorkDir) String() string {
	return string(f)
}

// A Client represents a user of the git command inside of a git repo. It's
// usually something that is trying to manipulate the repo.
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

// Creates a new client with the given gitDir and workdir. If not specified,
// NewClient will walk the filesystem until it finds a .git directory to use.
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

// Returns the branchname of the HEAD branch, or the empty string if the
// HEAD pointer is invalid or in a detached head state.
func (c *Client) GetHeadBranch() Branch {
	refspec, err := SymbolicRefGet(c, SymbolicRefOptions{}, "HEAD")
	if err != nil {
		return ""
	}

	// refspec's stringer trims trailing newlines.
	return Branch(refspec.String())
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

// ResetWorkTree will replace all objects in c.WorkDir with the content from
// the index.
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
		if strings.Index(indexEntry.PathName.String(), "/") > 0 {
			os.MkdirAll(filepath.Dir(indexEntry.PathName.String()), 0755)
		}
		err = ioutil.WriteFile(indexEntry.PathName.String(), obj.GetContent(), os.FileMode(indexEntry.Mode))
		if err != nil {
			continue
		}
		os.Chmod(indexEntry.PathName.String(), os.FileMode(indexEntry.Mode))
	}
	return nil
}

// Return valid branches that a Client knows about.
func (c *Client) GetBranches() (branches []Branch, err error) {
	files, err := ioutil.ReadDir(c.GitDir.String() + "/refs/heads")
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		branches = append(branches, Branch("refs/heads/"+f.Name()))
	}
	return
}

// Create a new branch in the Client's git repository.
func (c *Client) CreateBranch(name string, commit Commitish) error {
	id, err := commit.CommitID(c)
	if err != nil {
		return err
	}

	return c.GitDir.WriteFile(File("refs/heads/"+name), []byte(id.String()), 0644)
}

// A Person is usually an Author, but might be a committer. It's someone
// with an email address.
type Person struct {
	Name, Email string

	// Optional time that this person should be serialized as. Causes
	// String() to be returned in commit format if specified
	Time *time.Time
}

func (p Person) String() string {
	if p.Time == nil {
		return fmt.Sprintf("%s <%s>", p.Name, p.Email)
	}
	_, tzoff := p.Time.Zone()
	// for some reason t.Zone() returns the timezone offset in seconds
	// instead of hours, so convert it to an hour format string
	tzStr := fmt.Sprintf("%+03d00", tzoff/(60*60))
	return fmt.Sprintf("%s <%s> %d %s", p.Name, p.Email, p.Time.Unix(), tzStr)

}

// Returns the author that should be used for a commit message.
// If time t is provided,
func (c *Client) GetAuthor(t *time.Time) Person {
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
	return Person{name, email, t}
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

// Writes an object into the Client's .git/objects/ directory. This will write
// the object loosely, and not use a packfile.
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

// Returns true if the file on the filesystem hashes to Sha1, (which is usually
// the hash from the index) to determine if the file is clean.
func (f IndexPath) IsClean(c *Client, s Sha1) bool {
	fi, err := f.FilePath(c)
	if err != nil {
		panic(err)
		// return false instead?
	}
	if !fi.Exists() {
		return s == Sha1{}
	}
	fs, _, err := HashFile("blob", fi.String())
	if err != nil {
		panic(err)
	}
	return fs == s
}

// Gets the Commit of the current HEAD as a string.
func (c *Client) GetHeadCommit() (CommitID, error) {
	// If it's a symbolic ref, dereference it
	refspec, err := SymbolicRefGet(c, SymbolicRefOptions{}, "HEAD")
	if err == nil {
		b := Branch(refspec.String())
		return b.CommitID(c)
	}
	// Otherwise, try and parse the detached HEAD state.
	f := c.GitDir.File("HEAD")
	val, err := f.ReadFirstLine()
	if err != nil {
		return CommitID{}, InvalidHead
	}
	return CommitIDFromString(val)

}
