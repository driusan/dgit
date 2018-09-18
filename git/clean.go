package git

import (
	"fmt"
	"os"
)

// Command line options which affect the behaviour of clean.
type CleanOptions struct {
	Directory bool
	Force     bool
	DryRun    bool
	Quiet     bool

	// Flags which affect exclusion logic
	ExcludePatterns   []string
	NoStandardExclude bool
	OnlyExcluded      bool

	// Not implemented
	Interactive bool
}

func Clean(c *Client, opts CleanOptions, files []File) error {
	lsopts := LsFilesOptions{
		Others:          true,
		Directory:       opts.Directory,
		ExcludeStandard: !opts.NoStandardExclude,
		ExcludePatterns: opts.ExcludePatterns,
		Ignored:         opts.OnlyExcluded,
	}
	paths, err := LsFiles(c, lsopts, files)
	if err != nil {
		return err
	}
	for _, entry := range paths {
		f, err := entry.PathName.FilePath(c)
		if err != nil {
			return err
		}
		if !opts.Quiet {
			fmt.Printf("Removing %v\n", f)
		}
		if opts.DryRun {
			continue
		}
		if opts.Directory {
			if err := os.RemoveAll(f.String()); err != nil {
				return err
			}
		} else {
			if err := os.Remove(f.String()); err != nil {
				return err
			}
		}
	}
	return nil
}
