package git

import (
	"fmt"
	"io"
)

// A BitSetter denotes an option for a flag which sets or unsets
// a bit. It determines both whether the bit should be set, and
// the value it should be set to.
type BitSetter struct {
	// If true, the bit should be modified
	Modify bool

	// Value to set the bit to if Modify is true.
	Value bool
}

type CacheInfo struct {
	Mode EntryMode
	Sha1 Sha1
	Path IndexPath
}

type UpdateIndexOptions struct {
	Add, Remove            bool
	Refresh, ReallyRefresh bool
	Quiet                  bool
	IgnoreSubmodules       bool
	IndexInfo              bool
	SkipworkTree           bool

	IndexVersion int

	Unmerged      bool
	IgnoreMissing bool

	Again           bool
	Unresolve       bool
	InfoOnly        bool
	ForceRemove     bool
	Replace         bool
	Verbose         bool
	NullTerminate   bool
	Chmod           BitSetter
	SplitIndex      BitSetter
	UntrackedCache  BitSetter
	SkipWorktree    BitSetter
	AssumeUnchanged BitSetter

	CacheInfo CacheInfo

	Stdin io.Reader

	// With the official git client "git add -v" saysing "remove 'foo'"
	// while "git update-index --verbose" says "add 'foo'" when removing
	// a file. This is a hack so that we can have the same behaviour without
	// having to duplicate code.
	correctRemoveMsg bool
}

// This implements the git update-index command. It updates the index
// passed as a parameter, and returns it. It does *not* write it to
// disk, that's the responsibility of the caller.
func UpdateIndex(c *Client, idx *Index, opts UpdateIndexOptions, files []File) (*Index, error) {
	for _, file := range files {
		ipath, err := file.IndexPath(c)
		if err != nil {
			return nil, err
		}
		if !file.Exists() {
			if opts.Remove {
				idx.RemoveFile(ipath)
				if opts.Verbose {
					if opts.correctRemoveMsg {
						fmt.Printf("remove '%v'\n", file.String())
					} else {
						fmt.Printf("add '%v'\n", file.String())
					}
				}
			} else {
				return nil, fmt.Errorf("%v does not exist and --remove not passed", file)
			}
		} else if err := idx.AddFile(c, file, opts.Add); err != nil {
			if !opts.Add {
				// This is making invalid assumptions that the only
				// thing that might go wrong is that createEntry was
				// false and the file isn't in the index
				return nil, fmt.Errorf("Can not add %v to index. Missing --add?", file)
			}
			// If add was true and something went else went wrong,
			// return the error
			return nil, err
		}
		if opts.Verbose {
			fmt.Printf("add '%v'\n", file)
		}
	}
	return idx, nil
}
