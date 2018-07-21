package git

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
)

type AddOptions struct {
	Verbose            bool
	DryRun             bool
	Force              bool
	Interactive        bool
	Patch              bool
	Edit               bool
	Update             bool
	All                bool
	IgnoreRemoval      bool
	IntentToAdd        bool
	Refresh            bool
	IgnoreErrors       bool
	IgnoreMissing      bool
	NoWarnEmbeddedRepo bool
	Chmod              BitSetter
}

// Add implements the "git add" plumbing command.
func Add(c *Client, opts AddOptions, files []File) (*Index, error) {
	if opts.Patch {
		diffs, err := DiffFiles(c, DiffFilesOptions{}, files)
		if err != nil {
			return nil, err
		}
		var patchbuf bytes.Buffer
		if err := GeneratePatch(c, DiffCommonOptions{Patch: true}, diffs, &patchbuf); err != nil {
			return nil, err
		}
		hunks, err := splitPatch(patchbuf.String(), false)
		if err != nil {
			return nil, err
		}
		hunks, err = filterHunks("stage this hunk", hunks)
		if err == userAborted {
			return nil, nil
		} else if err != nil {
			return nil, err
		}

		patch, err := ioutil.TempFile("", "addpatch")
		if err != nil {
			return nil, err
		}
		defer os.Remove(patch.Name())
		recombinePatch(patch, hunks)

		if !opts.DryRun {
			if err := Apply(c, ApplyOptions{Cached: true}, []File{File(patch.Name())}); err != nil {
				return nil, err
			}

		}
		return c.GitDir.ReadIndex()
	}

	if len(files) == 0 {
		if !opts.All && !opts.Update {
			return nil, fmt.Errorf("Nothing to add. Did you mean \"git add .\"")
		}
		if opts.Update || opts.All {
			// LsFiles by default only shows things from under the
			// current directory, but -u is supposed to update the
			// whole repo.
			files = []File{File(c.WorkDir)}
		}
	}

	// Start by using ls-files to convert directories to files, and
	// (eventually) ignore .gitignore (once ls-files supports
	// --exclude-standard.)
	lsOpts := LsFilesOptions{
		Deleted:  true,
		Modified: true,
		Others:   true,
	}
	if !opts.Force {
		lsOpts.ExcludeStandard = true
	}

	if opts.Update {
		lsOpts.Others = false
	}
	if opts.IgnoreRemoval {
		lsOpts.Deleted = false
	}

	fileIdxs, err := LsFiles(c, lsOpts, files)
	if err != nil {
		return nil, err
	}
	fles := make([]File, len(fileIdxs), len(fileIdxs))
	for i, f := range fileIdxs {
		file, err := f.PathName.FilePath(c)
		if err != nil {
			return nil, err
		}
		fles[i] = file
	}

	updateIndexOpts := UpdateIndexOptions{
		Add:     true,
		Remove:  true,
		Verbose: opts.Verbose,
		Refresh: opts.Refresh,
		Chmod:   opts.Chmod,

		correctRemoveMsg: true,
	}
	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		return nil, err
	}
	newidx, err := UpdateIndex(c, idx, updateIndexOpts, fles)
	if err != nil {
		return nil, err
	}

	if !opts.DryRun {
		f, err := c.GitDir.Create(File("index"))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return newidx, newidx.WriteIndex(f)
	}
	return newidx, nil
}
