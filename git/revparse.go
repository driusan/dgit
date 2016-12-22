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

func (pr ParsedRevision) Ancestors(c *Client) []CommitID {
	comm, err := pr.CommitID(c)
	if err != nil {
		return nil
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

// RevParse will parse a single revision into a Commit object. If it's a parameter to
// rev-parse such as --git-dir, it will return the value of "other" and an empty
// sha1 instead. excluded will be true if it was parsed in a way that it should
// be excluded (ie. the revision started with a ^)
func RevParseCommit(c *Client, opt *RevParseOptions, arg string) (sha1 CommitID, excluded bool, err error) {
	var sha string
	if arg[0] == '^' {
		sha = arg[1:]
		excluded = true
	} else {
		sha = arg
		excluded = false
	}
	if len(sha) == 40 {
		comm, serr := Sha1FromString(sha)
		err = serr
		sha1 = CommitID(comm)
		return
	}
	if r := SymbolicRefGet(c, sha); r != "" {
		comm, serr := c.GetSymbolicRefCommit(r)
		err = serr
		sha1 = comm
		return
	}
	sha1, err = c.GetBranchCommit(sha)
	return
}

// Implements "git rev-prse". This should be refactored in terms of RevParseCommit and cleaned up.
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
				if len(sha) == 40 {
					comm, err := Sha1FromString(sha)
					if err != nil {
						panic(err)
					}
					commits = append(commits, ParsedRevision{comm, exclude})
					continue
				}
				if r := SymbolicRefGet(c, sha); r != "" {
					comm, err := c.GetSymbolicRefCommit(r)
					if err != nil {
						err2 = err
					} else {
						commits = append(commits, ParsedRevision{Sha1(comm), exclude})
					}
					continue
				}
				comm, err := c.GetBranchCommit(sha)
				if err != nil {
					err2 = err
				} else {
					commits = append(commits, ParsedRevision{Sha1(comm), exclude})
				}
			}
		}
	}
	return
}
