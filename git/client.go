package git

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/driusan/dgit/zlib"
)

var NoGlobalConfig = fmt.Errorf("No global .gitconfig file exists")

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

// ReadFile reads a file under the GitDir and returns the contents.
func (g GitDir) ReadFile(f File) ([]byte, error) {
	return ioutil.ReadFile(g.File(f).String())
}

// WorkDir is the top level of the work directory of the current process, or
// the empty string if the --bare option is provided
type WorkDir File

func (f WorkDir) String() string {
	return string(f)
}

type objectLocation struct {
	loose    bool
	packfile File
	index    *PackfileIndexV2
	offset   int64
}

type fileish interface {
	io.Reader
	io.Seeker
	io.Closer
}

type shaRef struct {
	Sha1
	bool
}

// A Client represents a user of the git command inside of a git repo. It's
// usually something that is trying to manipulate the repo.
type Client struct {
	GitDir  GitDir
	WorkDir WorkDir

	// This is used by the git-read-tree test suite. The description from the
	// git man page is:
	//
	//        Currently for internal use only. Set a prefix which gives a path
	//        from above a repository down to its root. One use is to give
	//        submodules context about the superproject that invoked it.
	//
	// The reality as far as we're concerned is it changes the output error
	// message of read-tree slightly.
	SuperPrefix string

	// Cache of where this client has previously found existing objects
	objectCache map[Sha1]objectLocation

	objcache map[shaRef]GitObject

	// Cache of previous config lookups to avoid re-parsing.
	configCache               map[string]string
	localConfig, globalConfig *GitConfig
}

func (c *Client) Close() error {
	return nil
}

// Returns true if the repo is a bare repo.
func (c *Client) IsBare() bool {
	return c.GetConfig("core.bare") == "true"
}

// Returns true if the path is under the client's GitDir.
func (c *Client) IsInsideGitDir(path File) bool {
	abs, err := filepath.Abs(path.String())
	if err != nil {
		panic(err)
	}
	absgd, err := filepath.Abs(c.GitDir.String())
	if err != nil {
		panic(err)
	}
	return strings.HasPrefix(abs, absgd)
}

// Returns true if path is inside of the client's work tree.
func (c *Client) IsInsideWorkTree(path File) bool {
	if c.IsBare() || c.IsInsideGitDir(path) {
		return false
	}

	abs, err := filepath.Abs(path.String())
	if err != nil {
		panic(err)
	}
	abswt, err := filepath.Abs(c.WorkDir.String())
	if err != nil {
		panic(err)
	}
	return strings.HasPrefix(abs, abswt)

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

	// This could be a bare repository, let's check the configuration
	configFileName := filepath.Join(startPath, "config")

	configFile, err := os.Open(configFileName)
	if err != nil {
		return ""
	}
	config := ParseConfig(configFile)
	if err := configFile.Close(); err != nil {
		return ""
	}

	bareVar, _ := config.GetConfig("core.bare")
	if bareVar == "true" {
		return GitDir(startPath)
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
		if workdir == "" && strings.HasSuffix(gitdir.String(), "/.git") {
			workdir = WorkDir(strings.TrimSuffix(gitdir.String(), "/.git"))
		}
	}
	m := make(map[Sha1]objectLocation)
	return &Client{GitDir(gitdir), WorkDir(workdir), "", m, make(map[shaRef]GitObject), nil, nil, nil}, nil
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
	fpath := filepath.Join(gd.String(), f.String())
	dir := File(filepath.Dir(fpath))
	if !dir.Exists() {
		if err := os.MkdirAll(dir.String(), 0755); err != nil {
			return nil, err
		}
	}
	return os.Create(fpath)
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
func (c *Client) GetBranches() ([]Branch, error) {
	files := []string{}

	err := filepath.Walk(filepath.Join(c.GitDir.String(), "refs", "heads"),
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				files = append(files, path)
			}
			return nil
		})
	if err != nil {
		return nil, err
	}

	branches := []Branch{}
	for _, f := range files {
		branches = append(branches, Branch(f[len(c.GitDir.String())+1:]))
	}
	return branches, nil
}

// Return valid branches that a Client knows about.
func (c *Client) GetRemoteBranches() (branches []Branch, err error) {
	remotes, err := ioutil.ReadDir(c.GitDir.String() + "/refs/remotes")
	if err != nil {
		return nil, err
	}
	for _, d := range remotes {
		if d.IsDir() {
			brs, err := ioutil.ReadDir(c.GitDir.String() + "/refs/remotes/" + d.Name())
			if err != nil {
				continue
			}
			for _, b := range brs {
				branches = append(branches, Branch("refs/remotes/"+d.Name()+"/"+b.Name()))
			}
		}
	}
	return
}

// Create a new branch in the Client's git repository.
func (c *Client) CreateBranch(name string, commit Commitish) error {
	id, err := commit.CommitID(c)
	if err != nil {
		return err
	}

	if name == "HEAD" {
		return fmt.Errorf("fatal: 'HEAD' is not a valid branch name.")
	}

	// Create the file using File.Create first to ensure any parent directories are created.
	if err := c.GitDir.File(File("refs/heads/" + name)).Create(); err != nil {
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
	return fmt.Sprintf("%s <%s> %s", p.Name, p.Email, timeToGitTime(*p.Time))

}

func timeToGitTime(t time.Time) string {
	_, tzoff := t.Zone()
	// for some reason t.Zone() returns the timezone offset in seconds
	// instead of hours, so convert it to an hour format string
	tzStr := fmt.Sprintf("%+03d00", tzoff/(60*60))
	val := fmt.Sprintf("%d %s", t.Unix(), tzStr)
	return val
}

// Returns the author that should be used for a commit message.
// If time t is provided,
func (c *Client) GetAuthor(t *time.Time) Person {
	person := Person{}
	name := os.Getenv("GIT_AUTHOR_NAME")
	email := os.Getenv("GIT_AUTHOR_EMAIL")

	// git config
	if name == "" {
		name = c.GetConfig("user.name")
	}
	if email == "" {
		email = c.GetConfig("user.email")
	}

	// system defaults
	if name == "" || email == "" {
		u, err := user.Current()
		if err != nil {
			panic(err)
		}

		if name == "" {
			name = u.Name
		}

		if email == "" {
			h, err := os.Hostname()
			if err != nil {
				panic(err)
			}
			email = fmt.Sprintf("%s@%s", u.Name, h)
		}
	}
	person.Name = name
	person.Email = email
	person.Time = t
	return person
}

// Returns the author that should be used for a commit message.
// If time t is provided,
func (c *Client) GetCommitter(t *time.Time) (Person, error) {
	person := Person{}
	name := os.Getenv("GIT_COMMITTER_NAME")
	email := os.Getenv("GIT_COMMITTER_EMAIL")
	var configerr error

	// git config
	if name == "" {
		name = c.GetConfig("user.name")
	}
	if email == "" {
		email = c.GetConfig("user.email")
	}

	// system defaults
	if name == "" || email == "" {
		u, err := user.Current()
		if err != nil {
			panic(err)
		}

		if name == "" {
			name = u.Name
		}

		if email == "" {
			h, err := os.Hostname()
			if err != nil {
				panic(err)
			}
			email = fmt.Sprintf("%s@%s", u.Name, h)
		}
		configerr = NoGlobalConfig
	}
	person.Name = name
	person.Email = email
	person.Time = t
	return person, configerr
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

	if have, _, err := c.HaveObject(Sha1(sha)); have == true || err != nil {
		if err != nil {
			return Sha1{}, err

		}
		return Sha1(sha), nil
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
		return false
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

// Determine whether or not the object represented by id exists in the
// Client's object directory. Returns a bool if it was found, and the
// basename of the packfile pack/idx pair that it was contained in (the
// zero value if it's stored loosely in the repo), and possibly an error
// if anything went wrong.
func (c *Client) HaveObject(id Sha1) (found bool, packedfile File, err error) {
	// If it's cached, avoid the overhead
	if val, ok := c.objectCache[id]; ok {
		return true, val.packfile, nil
	}

	// First the easy case
	if f := c.GitDir.File(File(fmt.Sprintf("objects/%02x/%018x", id[0], id[1:]))); f.Exists() {
		c.objectCache[id] = objectLocation{true, "", nil, 0}
		return true, "", nil
	}

	// Then, check if it's in a pack file.
	files, err := ioutil.ReadDir(c.GitDir.File("objects/pack").String())
	if err != nil {
		// The pack directory doesn't exist. It's not an error, but it definitely
		// doesn't have the file..
		log.Printf("No pack directory for object %s\n", id)
		return false, "", nil
	}
	for _, fi := range files {
		if filepath.Ext(fi.Name()) == ".idx" {
			// It's ambiguous if Name() has the full path or not according to what
			// ReadDir returns, so just be very cautious on how we open it.
			name := c.GitDir.File(File(fmt.Sprintf("objects/pack/%s", filepath.Base(fi.Name()))))
			f, err := os.Open(name.String())
			if err != nil {
				log.Print(err)
				continue
			}
			pfile := File(strings.TrimSuffix(name.String(), ".idx"))
			buf := bufio.NewReader(f)
			if v2PackIndexHasSha1(c, pfile, buf, id) {
				// We want to return the pack file, not the index.
				f.Close()
				return true, pfile, nil
			}
			f.Close()
		}
	}
	log.Printf("None of the pack files has object %s\n", id)
	return false, "", nil
}

// Sets a cached config for this session only. None of these configs
// will be persisted into the local or global configuration. Once the
// client is closed or is garbage collected the configuration is lost.
func (c *Client) SetCachedConfig(varname string, value string) {
	if c.configCache == nil {
		c.configCache = make(map[string]string)
	}

	c.configCache[varname] = value
}

// Gets a cached config variable if it is there. Otherwise, it returns
//  and empty string.
func (c *Client) GetCachedConfig(varname string) string {
	if c.configCache == nil {
		return ""
	}

	return c.configCache[varname]
}

// Loads a config variable for the git repo hosted by c.
// If the local variable exists, it will be used, otherwise
// it will use the global variable.
// Non-existent variables will return the empty string.
func (c *Client) GetConfig(varname string) string {
	if c.configCache == nil {
		c.configCache = make(map[string]string)
	}
	if val, ok := c.configCache[varname]; ok {
		return val
	}
	if c.localConfig == nil {
		config, err := LoadLocalConfig(c)
		if err == nil {
			c.localConfig = &config
		} else {
			// If there was an error we store
			// a non-nil empty value in the localconfig
			// cache to prevent it from being re-parsed
			c.localConfig = &GitConfig{}
		}
	}
	if val, _ := c.localConfig.GetConfig(varname); val != "" {
		c.configCache[varname] = val
		return val
	}
	if c.globalConfig == nil {
		config, err := LoadGlobalConfig()
		if err == nil {
			c.globalConfig = &config
		} else {
			// If there was an error we store
			// a non-nil empty value in the localconfig
			// cache to prevent it from being re-parsed
			c.globalConfig = &GitConfig{}
		}
	}
	if val, _ := c.globalConfig.GetConfig(varname); val != "" {
		c.configCache[varname] = val
		return val
	}
	return ""
}
