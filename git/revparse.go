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

	// Check if it's a symbolic ref
	if r, err := SymbolicRefGet(c, SymbolicRefOptions{}, SymbolicRef(arg)); err == nil {
		// It was a symbolic ref, convert it to a branch so that it's
		// a treeish.
		if b := Branch(r); b.Exists(c) {
			return b, nil
		}
	}

	if rs := c.GitDir.File("refs/tags/" + File(arg)); rs.Exists() {
		return RefSpec("refs/tags/" + arg), nil
	}
	// arg was not a Sha or a symbolic ref, it might still be a branch.
	// (This will return an error if arg is an invalid branch.)
	if b, err := GetBranch(c, arg); err == nil {
		return b, nil
	}
	return nil, fmt.Errorf("Invalid or unhandled treeish format.")
}

// RevParse will parse a single revision into a Commitish object.
func RevParseCommitish(c *Client, opt *RevParseOptions, arg string) (Commitish, error) {
	if len(arg) == 40 {
		sha1, err := Sha1FromString(arg)
		return CommitID(sha1), err
	}

	// Check if it's a symbolic ref
	var b Branch
	r, err := SymbolicRefGet(c, SymbolicRefOptions{}, SymbolicRef(arg))
	if err == nil {
		// It was a symbolic ref, convert the refspec to a branch.
		if b = Branch(r); b.Exists(c) {
			return b, nil
		}
	}
	if rs := c.GitDir.File("refs/tags/" + File(arg)); rs.Exists() {
		return RefSpec("refs/tags/" + arg), nil
	}

	// arg was not a Sha or a valid symbolic ref, it might still be a branch
	return GetBranch(c, arg)
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
		default:
			if len(arg) > 0 && arg[0] == '-' {
				fmt.Printf("%s\n", arg)
			} else {
				var sha string
				var exclude bool
				var err error
				if arg[0] == '^' {
					sha = arg[1:]
					exclude = true
				} else {
					sha = arg
					exclude = false
				}
				cmt, err := RevParseCommit(c, &opt, sha)
				if err != nil {
					err2 = err
				} else {
					commits = append(commits, ParsedRevision{Sha1(cmt), exclude})
				}
			}
		}
	}
	return
}
