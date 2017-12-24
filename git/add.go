package git

import (
	"fmt"
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
func Add(c *Client, opts AddOptions, files []File) error {
	if len(files) == 0 {
		if !opts.All && !opts.Update {
			return fmt.Errorf("Nothing to add. Did you mean \"git add .\"")
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
		return err
	}
	fles := make([]File, len(fileIdxs), len(fileIdxs))
	for i, f := range fileIdxs {
		file, err := f.PathName.FilePath(c)
		if err != nil {
			return err
		}
		fles[i] = file
	}

	updateIndexOpts := UpdateIndexOptions{
		Add:     true,
		Remove:  true,
		Verbose: opts.Verbose,
		Refresh: opts.Refresh,
		Chmod:   opts.Chmod,
	}
	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		return err
	}
	newidx, err := UpdateIndex(c, idx, updateIndexOpts, fles)
	if err != nil {
		return err
	}

	if !opts.DryRun {
		f, err := c.GitDir.Create(File("index"))
		if err != nil {
			return err
		}
		defer f.Close()
		return newidx.WriteIndex(f)
	}
	return nil
}
