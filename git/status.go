package git

import (
	"fmt"
	"os"
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
	if opts.Porcelain > 1 || opts.ShowStash || opts.Verbose || opts.Ignored || (opts.Column != "default" && opts.Column != "") {
		return "", fmt.Errorf("Unsupported option for Status")
	}
	if opts.Column == "" {
		opts.Column = "column"
	}
	var ret string
	if opts.Branch || opts.Long {
		branch, err := StatusBranch(c, opts, "")
		if err != nil {
			return "", err
		}
		if branch != "" {
			ret += branch + "\n"
		}
	}
	if opts.Short || opts.Porcelain == 1 || opts.NullTerminate {
		opts.Short = true // If porcelain=1, ensure short=true too..
		lineending := "\n"
		if opts.NullTerminate {
			lineending = "\000"
		}
		status, err := StatusShort(c, files, opts.UntrackedMode, "", lineending)
		if err != nil {
			return "", err
		}
		ret += status
	} else if opts.Long {
		status, err := StatusLong(c, files, opts.UntrackedMode, "")
		if err != nil {
			return "", err
		}
		ret += status
	}
	return ret, nil
}

func StatusBranch(c *Client, opts StatusOptions, lineprefix string) (string, error) {
	var ret string
	if opts.Short || opts.Porcelain > 0 {
		if !opts.Branch {
			return "", nil
		}
	}
	h, herr := c.GetHeadCommit()

	switch branch, err := SymbolicRefGet(c, SymbolicRefOptions{Short: true}, "HEAD"); err {
	case nil:
		if opts.Short {
			if herr != nil {
				return "## No commits yet on " + branch.String(), nil
			}
			return "## " + branch.String(), nil
		}
		ret = fmt.Sprintf("On branch %v", branch)
	case DetachedHead:
		if opts.Short {
			return "## HEAD (no branch)", nil
		}
		ret = fmt.Sprintf("HEAD detached at %v", h.String())
	default:
		return "", err
	}

	if herr != nil {
		ret += lineprefix + "\n\nNo commits yet\n"
	}
	return ret, nil

}

// Return a string of the status
func StatusLong(c *Client, files []File, untracked StatusUntrackedMode, lineprefix string) (string, error) {
	// If no head commit: "no changes yet", else branch info
	// Changes to be committed: dgit diff-index --cached HEAD
	// Unmerged: git ls-files -u
	// Changes not staged: dgit diff-files
	// Untracked: dgit ls-files -o
	var ret string
	index, _ := c.GitDir.ReadIndex()
	hasStaged := false

	var lsfiles []File
	if len(files) == 0 {
		lsfiles = []File{File(c.WorkDir)}
	} else {
		lsfiles = files
	}
	// Start by getting a list of unmerged and keeping them in a map, so
	// that we can exclude them from the non-"unmerged"
	unmergedMap := make(map[File]bool)
	unmerged, err := LsFiles(c, LsFilesOptions{Unmerged: true}, lsfiles)
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
	hasCommit := false
	if head, err := c.GetHeadCommit(); err != nil {
		// There is no head commit to compare against, so just say
		// everything in the cache (which isn't unmerged) is new
		staged, err := LsFiles(c, LsFilesOptions{Cached: true}, lsfiles)
		if err != nil {
			return "", err
		}
		var stagedMsg string
		if len(staged) > 0 {
			hasStaged = true
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
			ret += fmt.Sprintf("%v  (use \"git rm --cached <file>...\" to unstage)\n", lineprefix)
			ret += fmt.Sprintf("%v\n", lineprefix)
			ret += stagedMsg
			ret += fmt.Sprintf("%v\n", lineprefix)
		}
	} else {
		hasCommit = true
		staged, err = DiffIndex(c, DiffIndexOptions{Cached: true}, index, head, files)
		if err != nil {
			return "", err
		}
	}

	// Staged
	if len(staged) > 0 {
		hasStaged = true

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
						ret += fmt.Sprintf("%v\tboth modified:\t%v\n", lineprefix, fname)
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
	notstaged, err := DiffFiles(c, DiffFilesOptions{}, lsfiles)
	if err != nil {
		return "", err
	}

	hasUnstaged := false
	if len(notstaged) > 0 {
		hasUnstaged = true
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

	hasUntracked := false
	if untracked != StatusUntrackedNo {
		lsfilesopts := LsFilesOptions{
			Others:          true,
			ExcludeStandard: true, // Configurable some day
		}
		if untracked == StatusUntrackedNormal {
			lsfilesopts.Directory = true
		}

		untracked, err := LsFiles(c, lsfilesopts, lsfiles)
		if len(untracked) > 0 {
			hasUntracked = true
		}
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
	} else {
		if hasUnstaged {
			ret += fmt.Sprintf("%vUntracked files not listed (use -u option to show untracked files)\n", lineprefix)
		}
	}
	var summary string
	switch {
	case hasStaged && hasUntracked && hasCommit:
	case hasStaged && hasUntracked && !hasCommit:
	case hasStaged && !hasUntracked && hasCommit && !hasUnstaged:
	case hasStaged && !hasUntracked && hasCommit && hasUnstaged:
		if untracked != StatusUntrackedNo {
			summary = `no changes added to commit (use "git add" and/or "git commit -a")`
		}
	case hasStaged && !hasUntracked && !hasCommit:
	case !hasStaged && hasUntracked && hasCommit:
		fallthrough
	case !hasStaged && hasUntracked && !hasCommit:
		summary = `nothing added to commit but untracked files present (use "git add" to track)`
	case !hasStaged && !hasUntracked && hasCommit && !hasUnstaged:
		summary = "nothing to commit, working tree clean"
	case !hasStaged && !hasUntracked && hasCommit && hasUnstaged:
		summary = `no changes added to commit (use "git add" and/or "git commit -a")`
	case !hasStaged && !hasUntracked && !hasCommit:
		summary = `nothing to commit (create/copy files and use "git add" to track)`
	default:
	}
	if summary != "" {
		ret += lineprefix + summary + "\n"
	}
	return ret, nil
}

// Implements git status --short
func StatusShort(c *Client, files []File, untracked StatusUntrackedMode, lineprefix, lineending string) (string, error) {
	var lsfiles []File
	if len(files) == 0 {
		lsfiles = []File{File(c.WorkDir)}
	} else {
		lsfiles = files
	}

	cfiles, err := LsFiles(c, LsFilesOptions{Cached: true}, lsfiles)
	if err != nil {
		return "", err
	}
	tree := make(map[IndexPath]*IndexEntry)
	// It's not an error to use "git status" before the first commit,
	// so discard the error
	if head, err := c.GetHeadCommit(); err == nil {
		i, err := LsTree(c, LsTreeOptions{FullTree: true, Recurse: true}, head, files)
		if err != nil {
			return "", err
		}

		// this should probably be an LsTreeMap library function, it would be
		// useful other places..
		for _, e := range i {
			tree[e.PathName] = e
		}
	}
	var ret string
	var wtst, ist rune
	for i, f := range cfiles {
		wtst = ' '
		ist = ' '
		fname, err := f.PathName.FilePath(c)
		if err != nil {
			return "", err
		}
		switch f.Stage() {
		case Stage0:
			if head, ok := tree[f.PathName]; !ok {
				ist = 'A'
			} else {
				if head.Sha1 == f.Sha1 {
					ist = ' '
				} else {
					ist = 'M'
				}
			}

			stat, err := fname.Stat()
			if os.IsNotExist(err) {
				wtst = 'D'
			} else {
				mtime, err := fname.MTime()
				if err != nil {
					return "", err
				}
				if mtime != f.Mtime || stat.Size() != int64(f.Fsize) {
					wtst = 'M'
				} else {
					wtst = ' '
				}
			}
			if ist != ' ' || wtst != ' ' {
				ret += fmt.Sprintf("%c%c %v%v", ist, wtst, fname, lineending)
			}
		case Stage1:
			switch cfiles[i+1].Stage() {
			case Stage2:
				if i >= len(cfiles)-2 {
					// Stage3 is missing, we've reached the end of the index.
					ret += fmt.Sprintf("MD %v%v", fname, lineending)
					continue
				}
				switch cfiles[i+2].Stage() {
				case Stage3:
					// There's a stage1, stage2, and stage3. If they weren't all different, read-tree would
					// have resolved it as a trivial stage0 merge.
					ret += fmt.Sprintf("UU %v%v", fname, lineending)
				default:
					// Stage3 is missing, but we haven't reached the end of the index.
					ret += fmt.Sprintf("MD%v%v", fname, lineending)
				}
				continue
			case Stage3:
				// Stage2 is missing
				ret += fmt.Sprintf("DM %v%v", fname, lineending)
				continue
			default:
				panic("Unhandled index")
			}
		case Stage2:
			if i == 0 || cfiles[i-1].Stage() != Stage1 {
				// If this is a Stage2, and the previous wasn't Stage1,
				// then we know the next one must be Stage3 or read-tree
				// would have handled it as a trivial merge.
				ret += fmt.Sprintf("AA %v%v", fname, lineending)
			}
			// If the previous was Stage1, it was handled by the previous
			// loop iteration.
			continue
		case Stage3:
			// There can't be just a Stage3 or read-tree would
			// have resolved it as Stage0. All cases were handled
			// by Stage1 or Stage2
			continue
		}
	}
	if untracked != StatusUntrackedNo {
		lsfilesopts := LsFilesOptions{
			Others: true,
		}
		if untracked == StatusUntrackedNormal {
			lsfilesopts.Directory = true
		}

		untracked, err := LsFiles(c, lsfilesopts, lsfiles)
		if err != nil {
			return "", err
		}
		for _, f := range untracked {
			fname, err := f.PathName.FilePath(c)
			if err != nil {
				return "", err
			}
			if name := fname.String(); name == "." {
				ret += "?? ./" + lineending
			} else {
				ret += "?? " + name + lineending
			}
		}
	}
	return ret, nil

}
