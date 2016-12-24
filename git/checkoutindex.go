package git

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

// CheckoutIndexOptions represents the options that may be passed to
// "git checkout-index"
type CheckoutIndexOptions struct {
	UpdateStat bool

	Quiet bool
	Force bool
	All   bool

	NoCreate bool

	Prefix string

	// Stage not implemented
	Stage string // <number>|all

	// Temp not implemented
	Temp bool

	// Stdin implies checkout-index with the --stdin parameter.
	// nil implies it wasn't passed.
	// (Which is a moot point, because --stdin isn't implemented)
	Stdin         io.Reader // nil implies no --stdin param passed
	NullTerminate bool
}

// Same as "git checkout-index", except the Index is passed as a parameter (and
// may not have been written to disk yet). You likely want CheckoutIndex instead.
//
// (This is primarily for read-tree to be able to update the filesystem with the
// -u parameter.)
func CheckoutIndexUncommited(c *Client, idx *Index, opts CheckoutIndexOptions, files []string) error {
	if opts.All {
		for _, entry := range idx.Objects {
			files = append(files, entry.PathName.String())
		}
	}

	for _, entry := range idx.Objects {
		for _, file := range files {
			indexpath, err := File(file).IndexPath(c)
			if err != nil {
				if !opts.Quiet {
					fmt.Fprintf(os.Stderr, "%v\n", err)
				}
				continue

			}

			if entry.PathName != indexpath {
				continue
			}

			f := File(opts.Prefix + file)
			obj, err := c.GetObject(entry.Sha1)
			if f.Exists() && !opts.Force {
				if !opts.Quiet {
					fmt.Fprintf(os.Stderr, "%v already exists, no checkout\n", indexpath)
				}
				continue
			}
			if err != nil {
				return err
			}

			if !opts.NoCreate {
				fmode := os.FileMode(entry.Mode)
				err := ioutil.WriteFile(f.String(), obj.GetContent(), fmode)
				if err != nil {
					return err
				}
				os.Chmod(file, os.FileMode(entry.Mode))
			}

			// Update the stat information, but only if it's the same
			// file name. We only change the mtime, because the only
			// other thing we track is the file size, and that couldn't
			// have changed.
			// Don't change the stat info if there's a prefix, because
			// if we're checkout out into a prefix, it means we haven't
			// touched the index.
			if opts.UpdateStat && opts.Prefix == "" {
				fstat, err := f.Stat()
				if err != nil {
					return err
				}

				modTime := fstat.ModTime()
				entry.Mtime = uint32(modTime.Unix())
				entry.Mtimenano = uint32(modTime.Nanosecond())
			}
		}
	}

	if opts.UpdateStat {
		f, err := c.GitDir.Create(File("index"))
		if err != nil {
			return err
		}
		defer f.Close()
		return idx.WriteIndex(f)

	}
	return nil
}

// Implements the "git checkout-index" subcommand of git.
func CheckoutIndex(c *Client, opts CheckoutIndexOptions, files []string) error {
	if len(files) != 0 && opts.All {
		return fmt.Errorf("Can not mix --all and named files")
	}

	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		return err
	}
	return CheckoutIndexUncommited(c, idx, opts, files)
}
