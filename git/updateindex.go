package git

import (
	"bufio"
	"fmt"
	"io"
	"strings"
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
	SkipworkTree           bool

	IndexVersion int

	// Read the index information from IndexInfo
	IndexInfo io.Reader

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
	if opts.IndexInfo != nil {
		return UpdateIndexFromReader(c, opts, opts.IndexInfo)
	}
	if opts.Refresh {
		return UpdateIndexRefresh(c, idx, opts)
	}
	for _, file := range files {
		ipath, err := file.IndexPath(c)
		if err != nil {
			return nil, err
		}
		if file.IsDir() {
			if opts.Remove || opts.ForceRemove {
				entries, err := LsFiles(c, LsFilesOptions{Cached: true}, nil)
				if err != nil {
					return nil, err
				}
				for _, i := range entries {
					idx.RemoveFile(i.PathName)
				}
			} else if opts.Add {
				return nil, fmt.Errorf("%v is a directory - add files inside instead", file)
			}
		} else if !file.Exists() || opts.ForceRemove {
			if opts.Remove || opts.ForceRemove {
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
		} else if err := idx.AddFile(c, file, opts); err != nil {
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

func UpdateIndexFromReader(c *Client, opts UpdateIndexOptions, r io.Reader) (*Index, error) {
	idx := NewIndex()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		tab := strings.Split(line, "\t")
		if len(tab) != 2 {
			return nil, fmt.Errorf("Invalid line in index-info: %v", line)
		}
		path := tab[1]
		spaces := strings.Split(tab[0], " ")
		switch len(spaces) {
		case 2:
			// mode SP sha1 TAB path
			return nil, fmt.Errorf("update-index --index-info variant 1 not implemented")
		case 3:
			switch len(spaces[1]) {
			case 40:
				// mode SP sha1 SP stage TAB path
				mode, err := ModeFromString(spaces[0])
				if err != nil {
					return nil, err
				}
				sha1, err := Sha1FromString(spaces[1])
				if err != nil {
					return nil, err
				}
				var stage Stage
				switch spaces[2] {
				case "0":
					stage = Stage0
				case "1":
					stage = Stage1
				case "2":
					stage = Stage2
				case "3":
					stage = Stage3
				default:
					return nil, fmt.Errorf("Invalid stage: %v", spaces[2])
				}
				if err := idx.AddStage(c, IndexPath(path), mode, sha1, stage, 0, 0, UpdateIndexOptions{Add: true}); err != nil {
					return nil, err
				}
			default:
				// mode SP type SP sha1 TAB path
				mode, err := ModeFromString(spaces[0])
				if err != nil {
					return nil, err
				}
				if mode.TreeType() != spaces[1] {
					return nil, fmt.Errorf("Mode does not match type: %v", line)
				}
				sha1, err := Sha1FromString(spaces[2])
				if err != nil {
					return nil, err
				}
				if err := idx.AddStage(c, IndexPath(path), mode, sha1, Stage0, 0, 0, UpdateIndexOptions{Add: true}); err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("Invalid line in index-info: %v", line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return idx, nil
}

func UpdateIndexRefresh(c *Client, idx *Index, opts UpdateIndexOptions) (*Index, error) {
	for _, entry := range idx.Objects {
		if err := entry.RefreshStat(c); err != nil {
			return nil, err
		}
	}
	return idx, nil
}
