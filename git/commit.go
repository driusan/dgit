package git

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

type CommitOptions struct {
	All   bool
	Patch bool

	ResetAuthor bool
	Amend       bool

	Date   time.Time
	Author Person

	Signoff           bool
	NoVerify          bool
	AllowEmpty        bool
	AllowEmptyMessage bool

	NoPostRewrite bool
	Include       bool
	Only          bool
	Quiet         bool

	CleanupMode string
	NoEdit      bool

	// Should be passed to CommitTree, which needs support first:
	// GPGSign GPGKeyID
	// NoGpgSign bool

	// Things that are used to create the commit message and need to be
	// parsed by package cmd/, but not included here.
	//	ReuseMessage, ReeditMessage, Fixup, Squash Commitish
	// File string
	// Message string (-m)
	// Template File (COMMIT_EDITMSG)
	// Status, NoStatus bool
	// Verbose bool
	//

	// Things that only affect the output with --dry-run.
	// Note: Printing the status after --dry-run isn't implemented,
	// all it does is prevent the call to UpdateRef after CommitTree.
	// Most of these are a no-op.
	DryRun        bool
	Short         bool
	Branch        bool
	Porcelain     bool
	Long          bool
	NullTerminate bool
	UntrackedMode StatusUntrackedMode

	// FIXME: Add all the missing options here.
}

// Commit implements the command "git commit" in the repository pointed
// to by c.
func Commit(c *Client, opts CommitOptions, message CommitMessage, files []File) (CommitID, error) {
	if !opts.AllowEmptyMessage && message == "" {
		return CommitID{}, fmt.Errorf("Aborting commit due to empty commit message.")
	}
	if opts.Patch {
		return CommitID{}, fmt.Errorf("Commit --patch not implemented")
	}
	if len(files) != 0 {
		return CommitID{}, fmt.Errorf("Commit files not implemented")
	}
	if opts.All {
		var tostage []File
		if opts.Include {
			tostage = files
		}
		if _, err := Add(c, AddOptions{Update: true, DryRun: opts.DryRun}, tostage); err != nil {
			return CommitID{}, err
		}
	}

	// Happy path: write the tree
	treeid, err := WriteTree(c, WriteTreeOptions{})
	if err != nil {
		return CommitID{}, err
	}
	// Write the commit object
	var parents []CommitID
	oldHead, err := c.GetHeadCommit()
	if opts.Amend {
		parents, err = oldHead.Parents(c)
		if err != nil {
			return CommitID{}, err
		}
		author, err := oldHead.GetAuthor(c)
		if err != nil {
			return CommitID{}, err
		}
		if !opts.ResetAuthor {
			// Back up the environment variables that commit uses to
			// communicate with commit-tree, so that nothing external
			// changes to the caller of this script.
			defer func(oldauthorname, oldauthoremail, oldauthordate string) {
				os.Setenv("GIT_AUTHOR_NAME", oldauthorname)
				os.Setenv("GIT_AUTHOR_EMAIL", oldauthoremail)
				os.Setenv("GIT_AUTHOR_DATE", oldauthordate)

			}(
				os.Getenv("GIT_AUTHOR_NAME"),
				os.Getenv("GIT_AUTHOR_EMAIL"),
				os.Getenv("GIT_AUTHOR_DATE"),
			)
			os.Setenv("GIT_AUTHOR_NAME", author.Name)
			os.Setenv("GIT_AUTHOR_EMAIL", author.Email)
			date, err := oldHead.GetDate(c)
			if err != nil {
				return CommitID{}, err
			}
			os.Setenv("GIT_AUTHOR_DATE", date.Format("Mon, 02 Jan 2006 15:04:05 -0700"))
		}
		goto skipemptycheck
	} else if err == nil || err == DetachedHead {
		parents = append(parents, oldHead)
	}

	if !opts.AllowEmpty {
		if oldtree, err := oldHead.TreeID(c); err == nil {
			if oldtree == treeid {
				return CommitID{}, fmt.Errorf("No changes staged for commit.")
			}
		}
	}
skipemptycheck:
	cleanMessage, err := message.Cleanup(opts.CleanupMode, !opts.NoEdit)
	if err != nil {
		return CommitID{}, err
	}
	cid, err := CommitTree(c, CommitTreeOptions{}, TreeID(treeid), parents, cleanMessage)
	if err != nil {
		return CommitID{}, err
	}

	// Update the reference
	var refmsg string
	if len(cleanMessage) < 50 {
		refmsg = cleanMessage
	} else {
		refmsg = cleanMessage[:50]
	}
	refmsg = fmt.Sprintf("commit: %s (dgit)", refmsg)

	if err := UpdateRef(c, UpdateRefOptions{OldValue: oldHead, CreateReflog: true}, "HEAD", cid, refmsg); err != nil {
		return CommitID{}, err
	}
	return cid, nil
}

type CommitMessage string

func (cm CommitMessage) Cleanup(mode string, edit bool) (string, error) {
	switch mode {
	case "strip":
		return cm.strip(), nil
	case "whitespace":
		return cm.whitespace(), nil
	case "", "default":
		if edit {
			return cm.strip(), nil
		}
		return cm.whitespace(), nil
	case "scissors":
		return string(cm), fmt.Errorf("Unsupported cleanup mode")
	default:
		return string(cm), fmt.Errorf("Invalid cleanup mode")
	}
}

func (cm CommitMessage) whitespace() string {
	nonewlineRE, err := regexp.Compile("([\n]+)\n")
	if err != nil {
		panic(err)
	}
	replaced := nonewlineRE.ReplaceAllString(string(cm), "\n\n")
	return strings.TrimSpace(replaced) + "\n"
}

func (cm CommitMessage) strip() string {
	lines := strings.Split(cm.whitespace(), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) >= 1 && line[0] == '#' {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}
