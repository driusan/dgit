package git

import (
	"log"
	"sort"
)

// Options that are shared between git diff, git diff-files, diff-index,
// and diff-tree
type DiffCommonOptions struct {
	// Print a patch, not just the sha differences
	Patch bool

	// The 0 value implies 3.
	NumContextLines int

	// Generate the diff in raw format, not a unified diff
	Raw bool
}

// Describes the options that may be specified on the command line for
// "git diff-files". Note that only raw mode is currently supported, even
// though all the other options are parsed/set in this struct.
type DiffFilesOptions struct {
	DiffCommonOptions
}

// DiffFiles implements the git diff-files command.
// It compares the file system to the index.
func DiffFiles(c *Client, opt DiffFilesOptions, paths []File) ([]HashDiff, error) {
	indexentries, err := LsFiles(
		c,
		LsFilesOptions{
			Cached: true, Deleted: true, Modified: true,
		},
		paths,
	)
	if err != nil {
		return nil, err
	}

	var val []HashDiff

	for _, idx := range indexentries {
		fs := TreeEntry{}
		idxtree := TreeEntry{idx.Sha1, idx.Mode}

		f, err := idx.PathName.FilePath(c)
		if err != nil || !f.Exists() {
			// If there was an error, treat it as a non-existant file
			// and just use the empty Sha1
			val = append(val, HashDiff{idx.PathName, idxtree, fs, uint(idx.Fsize), 0})
			continue
		}
		stat, err := f.Lstat()
		if err != nil {
			val = append(val, HashDiff{idx.PathName, idxtree, fs, uint(idx.Fsize), 0})
			continue
		}

		switch {
		case stat.Mode().IsDir():
			// Since we're diffing files in the index (which only holds files)
			// against a directory, it means that the file was deleted and
			// replaced by a directory.
			val = append(val, HashDiff{idx.PathName, idxtree, fs, uint(idx.Fsize), 0})
			continue
		case !stat.Mode().IsRegular():
			// FIXME: This doesn't take into account that the file
			// might be some kind of non-symlink non-regular file.
			fs.FileMode = ModeSymlink
		case stat.Mode().Perm()&0100 != 0:
			fs.FileMode = ModeExec
		default:
			fs.FileMode = ModeBlob
		}
		mtime, err := f.MTime()
		if err != nil {
			return nil, err
		}
		size := stat.Size()
		ctm, ctmn := f.CTime()
		log.Printf("%v: Mtime %v idxmtime %v Size: %v idxsize: %v ctime: %v ctimenano:%v\n", f, mtime, idx.Mtime, size, idx.Fsize, idx.Ctime, idx.Ctimenano)
		if mtime != idx.Mtime || size != int64(idx.Fsize) || ctm != idx.Ctime || ctmn != idx.Ctimenano {
			val = append(val, HashDiff{idx.PathName, idxtree, fs, uint(idx.Fsize), uint(size)})
			continue
		}

		// The real git client appears to only compare lstat information and not hash the file. In fact,
		// if we hash the file then the official git test suite fails on the basic tests. Go, unfortunately,
		// only exposes mtime in an easy, cross-platform way, not ctime without going through the sys package
		if idx.Ctime == 0 {
			hash, _, err := HashFile("blob", f.String())

			if err != nil || hash != idx.Sha1 {
				val = append(val, HashDiff{idx.PathName, idxtree, fs, uint(idx.Fsize), uint(size)})
			}
		}
	}

	sort.Sort(ByName(val))

	return val, nil
}
