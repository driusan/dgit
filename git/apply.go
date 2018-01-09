package git

import (
	//	"fmt"
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
	// 1. Make a temporary directory to patch files in to ensure atomicity
	// 2. Copy files to tempdir
	// 3. Run an external patch tool
	// 4. If successful, copy the files back over WorkDir
	patchdir, err := ioutil.TempDir("", "gitapply")
	if err != nil {
		return err
	}
	defer os.RemoveAll(patchdir)

	// First pass, parse the patches to figure out which files are involved
	files := make(map[IndexPath]bool)
	for _, patch := range patches {
		patch, err := ioutil.ReadFile(patch.String())
		if err != nil {
			return err
		}
		hunks, err := splitPatch(string(patch), true)
		if err != nil {
			return err
		}
		for _, hunk := range hunks {
			files[hunk.File] = true
		}
	}

	// Copy all of the files. We do this in a second pass to void
	// needlessly recopying the same files multiple times.
	var idx *Index
	if opts.Cached {
		idx2, err := c.GitDir.ReadIndex()
		if err != nil {
			return err
		}
		idx = idx2
	}
	for file := range files {
		f, err := file.FilePath(c)
		if err != nil {
			return err
		}

		dst := patchdir + "/" + file.String()
		if opts.Cached {
			if err := copyFromIndex(c, idx, file, dst); err != nil {
				return err
			}
		} else {
			if err := copyFile(f.String(), dst); err != nil {
				return err
			}
		}
	}

	var patchDirection string
	if opts.Reverse {
		patchDirection = "-R"
	} else {
		patchDirection = "-N"
	}
	for _, patch := range patches {
		patchcmd := exec.Command(posixPatch, "--directory", patchdir, "-i", patch.String(), patchDirection, "-p1")
		patchcmd.Stderr = os.Stderr
		_, err := patchcmd.Output()
		if err != nil {
			return err
		}
	}
	if opts.Cached {
		return updateApplyIndex(c, idx, patchdir)
	}
	return copyApplyDir(c, patchdir)

}

// RestoreDir takes the directory dir, which is the directory that apply did
// its work in, and copies it back into the workdir.
func copyApplyDir(c *Client, dir string) error {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		relpath := strings.TrimPrefix(path, dir+"/")
		dstpath := c.WorkDir.String() + "/" + relpath
		return copyFile(path, dstpath)
	})
	return nil
}

// RestoreDir takes the directory dir, which is the directory that apply did
// its work in, and copies it back into the workdir.
func updateApplyIndex(c *Client, idx *Index, dir string) error {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		relpath := strings.TrimPrefix(path, dir+"/")
		contents, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		sha1, err := c.WriteObject("blob", contents)
		if err != nil {
			return err
		}

		ipath := IndexPath(relpath)
		// Avoid using AddStage so that mtime doesn't get modified,
		// and cause false negatives for file status
		for _, entry := range idx.Objects {
			if entry.PathName != ipath {
				continue
			}
			entry.Sha1 = sha1
			return nil
		}
		return nil
	})
	// Write the index that the callback modified
	f, err := c.GitDir.Create(File("index"))
	if err != nil {
		return err
	}
	defer f.Close()
	return idx.WriteIndex(f)
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	// Ensure that the directory exists for dst.
	dir := filepath.Dir(dst)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Create/truncate the file and copy
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return err
}

func copyFromIndex(c *Client, idx *Index, file IndexPath, dst string) error {
	sha1 := idx.GetSha1(file)
	obj, err := c.GetObject(sha1)
	if err != nil {
		return err
	}

	// Ensure that the directory exists for dst.
	dir := filepath.Dir(dst)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return ioutil.WriteFile(dst, obj.GetContent(), 0644)
}
