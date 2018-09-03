package git

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Pattern string

type ParsedRevision struct {
	Id       Sha1
	Excluded bool
}

func (pr ParsedRevision) CommitID(c *Client) (CommitID, error) {
	if pr.Id.Type(c) != "commit" {
		return CommitID{}, fmt.Errorf("Invalid revision commit")
	}
	return CommitID(pr.Id), nil
}

func (pr ParsedRevision) TreeID(c *Client) (TreeID, error) {
	if pr.Id.Type(c) != "commit" {
		return TreeID{}, fmt.Errorf("Invalid revision commit")
	}
	return CommitID(pr.Id).TreeID(c)
}

func (pr ParsedRevision) IsAncestor(c *Client, parent Commitish) bool {
	if pr.Id.Type(c) != "commit" {
		return false
	}
	com, err := pr.CommitID(c)
	if err != nil {
		return false
	}
	return com.IsAncestor(c, parent)
}

func (pr ParsedRevision) Ancestors(c *Client) ([]CommitID, error) {
	comm, err := pr.CommitID(c)
	if err != nil {
		return nil, err
	}
	return comm.Ancestors(c)
}

// Options that may be passed to RevParse on the command line.
// BUG(driusan): None of the RevParse options are implemented
type RevParseOptions struct {
	// Operation modes
	ParseOpt, SQQuote bool

	// Options for --parseopt
	KeepDashDash    bool
	StopAtNonOption bool
	StuckLong       bool

	// Options for Filtering
	RevsOnly       bool
	NoRevs         bool
	Flags, NoFlags bool

	// Options for output. These should probably not be here but be handled
	// in the cmd package instead.
	Default                    string
	Prefix                     string
	Verify                     bool
	Quiet                      bool
	SQ                         bool
	Not                        bool
	AbbrefRev                  string //strict|loose
	Short                      uint   // The number of characters to abbreviate to. Default should be "4"
	Symbolic, SymbolicFullName bool

	// Options for Objects
	All                     bool
	Branches, Tags, Remotes Pattern
	Glob                    Pattern
	Exclude                 Pattern
	Disambiguate            string // Prefix

	// Options for Files
	// BUG(driusan): These should be handled as part of "args", not in RevParseOptions.
	// They're included here so that I don't forget about them.
	GitDir           GitDir
	GitCommonDir     GitDir
	IsInsideGitDir   bool
	IsInsideWorkTree bool
	IsBareRepository bool
	ResolveGitDir    File // path
	GitPath          GitDir
	ShowCDup         bool
	ShowPrefix       bool
	ShowToplevel     bool
	SharedIndexPath  bool

	// Other options
	After, Before time.Time
}

// RevParsePath parses a path spec such as `HEAD:README.md` into the value that
// it represents. The Sha1 returned may be either a tree or a blob, depending on
// the pathspec.
func RevParsePath(c *Client, opt *RevParseOptions, arg string) (Sha1, error) {
	var tree Treeish
	var err error
	var treepart string
	pathcomponent := strings.Index(arg, ":")
	if pathcomponent < 0 {
		treepart = arg
	} else if pathcomponent == 0 {
		treepart = "HEAD"
	} else {
		treepart = arg[0:pathcomponent]
	}
	if len(arg) == 40 {
		comm, err := Sha1FromString(arg)
		if err != nil {
			goto notsha1
		}
		switch comm.Type(c) {
		case "blob":
			if pathcomponent >= 0 {
				// There was a path part, but there's no way for a path
				// to be in a blob.
				return Sha1{}, fmt.Errorf("Could not parse %v", arg)
			}
			return comm, nil
		case "tree":
			tree = TreeID(comm)
			goto extractpath
		case "commit":
			tree = CommitID(comm)
			goto extractpath
		default:
			return Sha1{}, fmt.Errorf("%s is not a valid sha1", arg)
		}
	}
notsha1:
	tree, err = RevParseTreeish(c, opt, treepart)
	if err != nil {
		return Sha1{}, err
	}
extractpath:
	if pathcomponent < 0 {
		treeid, err := tree.TreeID(c)
		if err != nil {
			return Sha1{}, err
		}
		return Sha1(treeid), nil
	}
	path := arg[pathcomponent+1:]
	indexes, err := expandGitTreeIntoIndexes(c, tree, true, true, false)
	for _, entry := range indexes {
		if entry.PathName == IndexPath(path) {
			return entry.Sha1, nil
		}
	}
	return Sha1{}, fmt.Errorf("%v not found", arg)
}

// RevParseTreeish will parse a single revision into a Treeish structure.
func RevParseTreeish(c *Client, opt *RevParseOptions, arg string) (Treeish, error) {
	if len(arg) == 40 {
		comm, err := Sha1FromString(arg)
		if err != nil {
			return nil, err
		}
		switch comm.Type(c) {
		case "tree":
			return TreeID(comm), nil
		case "commit":
			return CommitID(comm), nil
		default:
			return nil, fmt.Errorf("%s is not a tree-ish", arg)
		}
	}
	if arg == "HEAD" {
		return c.GetHeadCommit()
	}

	cid, err := RevParseCommitish(c, opt, arg)
	if err != nil {
		return nil, err
	}
	// A CommitID implements Treeish, so we just resolve the commitish to a real commit
	return cid.CommitID(c)
}

// RevParse will parse a single revision into a Commitish object.
func RevParseCommitish(c *Client, opt *RevParseOptions, arg string) (cmt Commitish, err error) {
	var cmtbase string
	if pos := strings.IndexAny(arg, "@^"); pos >= 0 {
		cmtbase = arg[:pos]
		defer func(mod string) {
			if err != nil {
				// If there was already an error, then just let it be.
				return
			}
			// FIXME: This should actually implement various ^ and @{} modifiers
			if mod == "^" {
				basecmt, newerr := cmt.CommitID(c)
				if newerr != nil {
					err = newerr
					return
				}
				parents, newerr := basecmt.Parents(c)
				if newerr != nil {
					err = newerr
					return
				}
				if len(parents) != 1 {
					err = fmt.Errorf("Can not use ^ modifier on merge commit.")
					return
				}
				cmt = parents[0]
				return
			}
			err = fmt.Errorf("Unhandled commit modifier: %v", mod)
		}(arg[pos:])
	} else {
		cmtbase = arg
	}
	if len(cmtbase) == 40 {
		sha1, err := Sha1FromString(cmtbase)
		return CommitID(sha1), err
	}
	if cmtbase == "HEAD" {
		return c.GetHeadCommit()
	}

	// Check if it's a symbolic ref
	var b Branch
	r, err := SymbolicRefGet(c, SymbolicRefOptions{}, SymbolicRef(cmtbase))
	if err == nil {
		// It was a symbolic ref, convert the refspec to a branch.
		if b = Branch(r); b.Exists(c) {
			return b, nil
		}
	}
	if rs := c.GitDir.File("refs/tags/" + File(cmtbase)); rs.Exists() {
		return RefSpec("refs/tags/" + cmtbase), nil
	}

	// arg was not a Sha or a symbolic ref, it might still be a branch.
	// (This will return an error if arg is an invalid branch.)
	if b, err := GetBranch(c, cmtbase); err == nil {
		return b, nil
	}
	return nil, fmt.Errorf("Could not parse %v", arg)
}

// RevParse will parse a single revision into a Commit object.
func RevParseCommit(c *Client, opt *RevParseOptions, arg string) (CommitID, error) {
	cmt, err := RevParseCommitish(c, opt, arg)
	if err != nil {
		return CommitID{}, fmt.Errorf("Invalid commit: %s", arg)
	}
	return cmt.CommitID(c)
}

// Implements "git rev-parse". This should be refactored in terms of RevParseCommit and cleaned up.
// (clean up a lot.)
func RevParse(c *Client, opt RevParseOptions, args []string) (commits []ParsedRevision, err2 error) {
	for _, arg := range args {
		switch arg {
		case "--git-dir":
			wd, err := os.Getwd()
			if err == nil {
				fmt.Printf("%s\n", strings.TrimPrefix(c.GitDir.String(), wd+"/"))
			} else {
				fmt.Printf("%s\n", c.GitDir)
			}
		case "--verify":
			// FIXME: Implement this properly. This is just to prevent default
			// from outputing the string '--verify'
			continue
		default:
			if len(arg) > 0 && arg[0] == '-' {
				fmt.Printf("%s\n", arg)
			} else {
				var sha string
				var exclude bool
				if arg[0] == '^' {
					sha = arg[1:]
					exclude = true
				} else {
					sha = arg
					exclude = false
				}
				if strings.Contains(arg, ":") {
					sha, err := RevParsePath(c, &opt, arg)
					if err != nil {
						err2 = err
					} else {
						commits = append(commits, ParsedRevision{sha, exclude})
					}
				} else {
					cmt, err := RevParseCommit(c, &opt, sha)
					if err != nil {
						err2 = err
					} else {
						commits = append(commits, ParsedRevision{Sha1(cmt), exclude})
					}
				}
			}
		}
	}
	return
}
