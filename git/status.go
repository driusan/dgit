package git

import (
	"fmt"
)

type StatusUntrackedMode uint8

const (
	StatusUntrackedNo = StatusUntrackedMode(iota)
	StatusUntrackedNormal
	StatusUntrackedAll
)

type StatusIgnoreSubmodules uint8

const (
	StatusIgnoreSubmodulesNone = StatusIgnoreSubmodules(iota)
	StatusIgnoreSubmodulesUntracked
	StatisIgnoreSubmodulesDirty
	StatusIgnoreSubmodulesAll
)

type StatusColumnOptions string

type StatusOptions struct {
	Short     bool
	Branch    bool
	ShowStash bool
	Porcelain uint8
	Long      bool
	Verbose   bool
	Ignored   bool

	NullTerminate bool

	UntrackedMode StatusUntrackedMode

	IgnoreSubmodules StatusIgnoreSubmodules
	Column           StatusColumnOptions
}

func Status(c *Client, opts StatusOptions, files []File) (string, error) {
	// It's not an error in status if there hasn't been a commit yet, so
	// discard the error.
	head, _ := c.GetHeadCommit()

	if opts.Short || opts.Porcelain > 0 || opts.ShowStash || opts.Verbose || opts.Ignored || opts.NullTerminate || opts.Column != "default" {
		return "", fmt.Errorf("Unsupported option for Status")
	}
	var ret string
	if opts.Branch || opts.Long {
		branch, err := StatusBranch(c, head, "")
		if err != nil {
			return "", err
		}
		ret += branch
	}
	if opts.Long {
		status, err := StatusLong(c, head, files, opts.UntrackedMode, "")
		if err != nil {
			return "", err
		}
		ret += status
	}
	return ret, nil
}

func StatusBranch(c *Client, head Commitish, lineprefix string) (string, error) {
	// FIXME: Implement this.
	return "", nil
}

// Return a string of the status
func StatusLong(c *Client, head Treeish, files []File, untracked StatusUntrackedMode, lineprefix string) (string, error) {
	// If no head commit: "no changes yet", else branch info
	// Changes to be committed: dgit diff-index --cached HEAD
	// Unmerged: git ls-files -u
	// Changes not staged: dgit diff-files
	// Untracked: dgit ls-files -o

	var ret string

	// Start by getting a list of unmerged and keeping them in a map, so
	// that we can exclude them from the non-"unmerged"
	unmergedMap := make(map[File]bool)
	unmerged, err := LsFiles(c, LsFilesOptions{Unmerged: true}, files)
	if err != nil {
		return "", err
	}
	for _, f := range unmerged {
		fname, err := f.PathName.FilePath(c)
		if err != nil {
			return "", err
		}
		unmergedMap[fname] = true
	}

	var staged []HashDiff
	var summary string
	if head == (CommitID{}) {
		// There is no head commit to compare against, so just say
		// everything in the cache (which isn't unmerged) is new
		staged, err := LsFiles(c, LsFilesOptions{Cached: true}, files)
		if err != nil {
			return "", err
		}
		var stagedMsg string
		if len(staged) > 0 {
			for _, f := range staged {
				fname, err := f.PathName.FilePath(c)
				if err != nil {
					return "", err
				}

				if _, ok := unmergedMap[fname]; ok {
					// There's a merge conflict, it'l show up in "Unmerged"
					continue
				}
				stagedMsg += fmt.Sprintf("%v\tnew file:\t%v\n", lineprefix, fname)
			}
		}

		if stagedMsg != "" {
			ret += fmt.Sprintf("%vChanges to be committed:\n", lineprefix)
			ret += fmt.Sprintf("%v  (use \"git reset HEAD <file>...\" to unstage)\n", lineprefix)
			ret += fmt.Sprintf("%v\n", lineprefix)
			ret += stagedMsg
			ret += fmt.Sprintf("%v\n", lineprefix)
		} else {
			// FIXME: This 1) isn't the right message if there's unmerged entries and
			//    2) should go at the end, not the beginning of the status.
			if len(unmerged) == 0 {
				summary = fmt.Sprintf("%vnothing to commit. (create/copy files and use \"git add\" to track)\n", lineprefix)
			} else {
				summary = fmt.Sprintf("%vno changes added to commit. (use \"git add\" and/or \"git commit -a\")\n", lineprefix)
			}
		}

	} else if head != nil {
		staged, err = DiffIndex(c, DiffIndexOptions{Cached: true}, head, files)
		if err != nil {
			return "", err
		}
	}

	// Staged
	if len(staged) > 0 {
		stagedMsg := ""
		for _, f := range staged {
			fname, err := f.Name.FilePath(c)
			if err != nil {
				return "", err
			}

			if _, ok := unmergedMap[fname]; ok {
				// There's a merge conflict, it'l show up in "Unmerged"
				continue
			}

			if f.Src == (TreeEntry{}) {
				stagedMsg += fmt.Sprintf("%v\tnew file:\t%v\n", lineprefix, fname)
			} else if f.Dst == (TreeEntry{}) {
				stagedMsg += fmt.Sprintf("%v\tdeleted:\t%v\n", lineprefix, fname)
			} else {
				stagedMsg += fmt.Sprintf("%v\tmodified:\t%v\n", lineprefix, fname)
			}
		}
		if stagedMsg != "" {
			ret += fmt.Sprintf("%vChanges to be committed:\n", lineprefix)
			ret += fmt.Sprintf("%v  (use \"git reset HEAD <file>...\" to unstage)\n", lineprefix)
			ret += fmt.Sprintf("%v\n", lineprefix)
			ret += stagedMsg
			ret += fmt.Sprintf("%v\n", lineprefix)
		}
	}

	// We already did the LsFiles for the unmerged, so just iterate over
	// them.
	if len(unmerged) > 0 {
		ret += fmt.Sprintf("%vUnmerged paths:\n", lineprefix)
		ret += fmt.Sprintf("%v  (use \"git reset HEAD <file>...\" to unstage)\n", lineprefix)
		ret += fmt.Sprintf("%v  (use \"git add <file>...\" to mark resolution)\n", lineprefix)
		ret += fmt.Sprintf("%v\n", lineprefix)

		for i, f := range unmerged {
			fname, err := f.PathName.FilePath(c)
			if err != nil {
				return "", err
			}
			switch f.Stage() {
			case Stage1:
				switch unmerged[i+1].Stage() {
				case Stage2:
					if i >= len(unmerged)-2 {
						// Stage3 is missing, we've reached the end of the index.
						ret += fmt.Sprintf("%v\tdeleted by them:\t%v\n", lineprefix, fname)
						continue
					}
					switch unmerged[i+2].Stage() {
					case Stage3:
						// There's a stage1, stage2, and stage3. If they weren't all different, read-tree would
						// have resolved it as a trivial stage0 merge.
						ret += fmt.Sprintf("%v\tmodified by both:\t%v\n", lineprefix, fname)
					default:
						// Stage3 is missing, but we haven't reached the end of the index.
						ret += fmt.Sprintf("%v\tdeleted by them:\t%v\n", lineprefix, fname)
					}
					continue
				case Stage3:
					// Stage2 is missing
					ret += fmt.Sprintf("%v\tdeleted by us:\t%v\n", lineprefix, fname)
					continue
				default:
					panic("Unhandled index")
				}
			case Stage2:
				if i == 0 || unmerged[i-1].Stage() != Stage1 {
					// If this is a Stage2, and the previous wasn't Stage1,
					// then we know the next one must be Stage3 or read-tree
					// would have handled it as a trivial merge.
					ret += fmt.Sprintf("%v\tboth added:\t%v\n", lineprefix, fname)
				}
				// If the previous was Stage1, it was handled by the previous
				// loop iteration.
				continue
			case Stage3:
				// There can't be just a Stage3 or read-tree would
				// have resolved it as Stage0. All cases were handled
				// by Stage1 or Stage2
				continue
			default:
				// If ls-files -u returned something other than
				// Stage1-3, there's an unrelated bug somewhere.
				panic("Invalid unmerged stage")
			}
		}
		ret += fmt.Sprintf("%v\n", lineprefix)
	}
	// Not staged changes
	notstaged, err := DiffFiles(c, DiffFilesOptions{}, files)
	if err != nil {
		return "", err
	}

	if len(notstaged) > 0 {
		notStagedMsg := ""
		for _, f := range notstaged {
			fname, err := f.Name.FilePath(c)
			if err != nil {
				return "", err
			}

			if _, ok := unmergedMap[fname]; ok {
				// There's a merge conflict, it'l show up in "Unmerged"
				continue
			}

			if f.Src == (TreeEntry{}) {
				notStagedMsg += fmt.Sprintf("%v\tnew file:\t%v\n", lineprefix, fname)
			} else if f.Dst == (TreeEntry{}) {
				notStagedMsg += fmt.Sprintf("%v\tdeleted:\t%v\n", lineprefix, fname)
			} else {
				notStagedMsg += fmt.Sprintf("%v\tmodified:\t%v\n", lineprefix, fname)
			}
		}
		if notStagedMsg != "" {
			ret += fmt.Sprintf("%vChanges not staged for commit:\n", lineprefix)
			ret += fmt.Sprintf("%v  (use \"git add <file>...\" to update what will be committed)\n", lineprefix)
			ret += fmt.Sprintf("%v  (use \"git checkout -- <file>...\" to discard changes in working directory)\n", lineprefix)
			ret += fmt.Sprintf("%v\n", lineprefix)
			ret += notStagedMsg
			ret += fmt.Sprintf("%v\n", lineprefix)
		}
	}

	if untracked != StatusUntrackedNo {
		lsfilesopts := LsFilesOptions{
			Others: true,
		}
		if untracked == StatusUntrackedNormal {
			lsfilesopts.Directory = true
		}

		untracked, err := LsFiles(c, lsfilesopts, files)
		if err != nil {
			return "", err
		}
		if len(untracked) > 0 {
			ret += fmt.Sprintf("%vUntracked files:\n", lineprefix)
			ret += fmt.Sprintf("%v  (use \"git add <file>...\" to include in what will be committed)\n", lineprefix)
			ret += fmt.Sprintf("%v\n", lineprefix)

			for _, f := range untracked {
				fname, err := f.PathName.FilePath(c)
				if err != nil {
					return "", err
				}
				if fname.IsDir() {
					ret += fmt.Sprintf("%v\t%v/\n", lineprefix, fname)
				} else {
					ret += fmt.Sprintf("%v\t%v\n", lineprefix, fname)
				}
			}
			ret += fmt.Sprintf("%v\n", lineprefix)
		}
	}
	if ret == "" {
		ret = fmt.Sprintf("%vnothing to commit, working tree clean\n", lineprefix)
	}
	ret += summary
	return ret, nil
}
