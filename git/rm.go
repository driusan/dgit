package git

import (
	"fmt"
)

// RmOptions denotes command line options that may
// be parsed from "git rm"
type RmOptions struct {
	Force           bool
	DryRun          bool
	Recursive       bool
	Cached          bool
	IgnoreUnmatched bool
	Quiet           bool
}

func Rm(c *Client, opts RmOptions, files []File) error {
	if !opts.Recursive {
		for _, f := range files {
			if f.IsDir() {
				return fmt.Errorf("Not removing %v recursively without -r", f)
			}
		}
	}
	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		return err
	}
	if !opts.IgnoreUnmatched {
		im := idx.GetMap()

		for _, f := range files {
			ip, err := f.IndexPath(c)
			if err != nil {
				return err
			}

			var found bool
			if opts.Recursive {
				found = im.Contains(ip)
			} else {
				_, found = im[ip]
			}

			if !found {
				return fmt.Errorf("pathspec %v did not match any files", f)
			}
		}
	}
	if !opts.Force {
		modified, err := LsFiles(c, LsFilesOptions{Modified: true}, files)
		if err != nil {
			return err
		}
		errors := ""
		for _, ip := range modified {
			f, err := ip.PathName.FilePath(c)
			if err != nil {
				return err
			}

			if errors == "" {
				errors = fmt.Sprintf("file %v has local modifications", f)
			} else {
				errors += fmt.Sprintf("\nfile %v has local modifications", f)
			}
		}
		if errors != "" {
			return fmt.Errorf("%s", errors)
		}
	}

	deleted, err := LsFiles(c, LsFilesOptions{Deleted: true}, files)
	if err != nil {
		return err
	}
	for _, ip := range deleted {
		f, err := ip.PathName.FilePath(c)
		if err != nil {
			return err
		}
		if !opts.Quiet {
			fmt.Printf("rm '%v'\n", f)
		}
		if opts.DryRun {
			continue
		}
		idx.RemoveFile(ip.PathName)
		if opts.Cached {
			continue
		}
		if err := f.Remove(); err != nil {
			return err
		}
	}
	return nil
}
