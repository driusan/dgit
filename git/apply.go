package git

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ApplyOptions struct {
	Stat, NumStat, Summary bool

	Check, Index, Cached bool

	ThreeWay bool

	BuildFakeAncestor string

	Reject bool

	NullTerminate bool

	Strip, Context int

	UnidiffZero bool

	ForceApply bool

	Reverse bool

	NoAdd bool

	ExcludePattern, IncludePattern string

	InaccurateEof bool

	Verbose bool

	Recount bool

	Directory string

	UnsafePaths bool

	Whitespace string
}

func Apply(c *Client, opts ApplyOptions, patches []File) error {
	// 1. Make backup dir to ensure patch is atomc and rollback if applicable
	// 2. Run an external posix patch command
	// 3. If it didn't succeed, restore from backup to ensure atomicity
	backupdir, err := ioutil.TempDir("", "gitapply")
	if err != nil {
		return err
	}
	//	defer os.RemoveAll(backupdir)

	for _, patch := range patches {
		patchcmd := exec.Command(posixPatch, "--backup", "-B", backupdir+"/", "--directory", c.WorkDir.String(), "-i", patch.String(), "-N", "-r", "foo.rej", "-p1", "-r", "/dev/null")
		patchcmd.Stderr = os.Stderr
		out, err := patchcmd.Output()
		fmt.Printf("Output: %s\n", out)
		if err != nil {
			if err := restoreDir(c, backupdir); err != nil {
				// If our err recovery errored out, we're in a really
				// bad state, so panic.
				panic(err)
			}
			fmt.Printf("Should restore %s\n", backupdir)
			return err
		}
	}
	return nil
}

// RestoreDir takes the directory dir, which is a backup dir created by apply,
// and copies it over c's WorkDir to overwrite any in-place backups that might
// have happened.
func restoreDir(c *Client, dir string) error {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		relpath := strings.TrimPrefix(path, dir+"/")
		dstpath := c.WorkDir.String() + "/" + relpath
		fmt.Printf("Copying %v to %v", path, dstpath)
		return copyFile(path, dstpath)
	})
	return nil
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return err
}
