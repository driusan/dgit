package git

import (
	"fmt"
)

type ResetOptions struct {
	Quiet bool

	Soft, Mixed, Hard, Merge, Keep bool
}

// Reset implementes the "git reset" command and delegates to the appropriate
// ResetMode or ResetUnstage subcommand.
//
// While the third option is a []File slice, the first argument may instead
// be a "filename" which parses to a commitish or treeish by rev-parse, in
// which case it's used as the base to reset to, not a file. In the event that
// there's both a commitish in git and filename of the same name, it's treated
// as a commit and you must instead pass "HEAD" as the first file to treat
// it as a filename.
//
// This behaviour is different than the official git client, which provides
// an error and a message to disambiguate based on the presence of "--" on
// the command line. (This is not an option here, because this is a library
// function, not a command line.)
func Reset(c *Client, opts ResetOptions, files []File) error {
	if opts.Soft && len(files) > 1 {
		return fmt.Errorf("Cannot do soft reset with paths")
	}
	if opts.Mixed {
		switch {
		case len(files) > 1:
			goto unstage
		case len(files) == 1:
			cmt, err := RevParseCommitish(c, &RevParseOptions{}, files[0].String())
			if err != nil {
				goto unstage
			}
			return ResetMode(c, opts, cmt)
		case len(files) == 0:
			cmt, err := c.GetHeadCommit()
			if err != nil {
				return err
			}
			return ResetMode(c, opts, cmt)
		}
	}
	if opts.Hard && len(files) > 1 {
		return fmt.Errorf("Cannot do hard reset with paths")
	}
	if opts.Merge && len(files) > 1 {
		return fmt.Errorf("Cannot do merge reset with paths")
	}
	if opts.Keep && len(files) > 1 {
		return fmt.Errorf("Cannot do keep reset with paths")
	}
	if opts.Soft || opts.Mixed || opts.Hard || opts.Merge || opts.Keep {
		if len(files) == 0 {
			head, err := c.GetHeadCommit()
			if err != nil {
				return err
			}
			return ResetMode(c, opts, head)
		} else if len(files) == 1 {
			cmt, err := RevParseCommitish(c, &RevParseOptions{}, files[0].String())
			if err != nil {
				return err
			}
			return ResetMode(c, opts, cmt)
		}
		// > 1 was handled above.
		panic("This line should be unreachable.")

	}
unstage:
	if len(files) == 0 {
		head, err := c.GetHeadCommit()
		if err != nil {
			return err
		}
		return ResetUnstage(c, opts, head, files)
	}
	treeish, err := RevParseTreeish(c, &RevParseOptions{}, files[0].String())
	if err != nil {
		treeish, err = c.GetHeadCommit()
		if err != nil {
			return err
		}
	} else {
		files = files[1:]
	}
	return ResetUnstage(c, opts, treeish, files)
}

// ResetMode implements "git reset [--soft | --mixed | --hard | --merge | --keep]" <commit>
func ResetMode(c *Client, opts ResetOptions, cmt Commitish) error {
	if !opts.Soft && !opts.Mixed && !opts.Hard && !opts.Merge && !opts.Keep {
		// The default mode is mixed is none were implemented.
		opts.Mixed = true
	}

	// Update HEAD to cmt
	// If soft -- nothing else
	// if mixed -- read-tree cmt
	// if hard -- read-tree cmt && checkout-index -f -all
	// if merge | keep-- read man page more carefully, for now return an error

	if opts.Merge {
		return fmt.Errorf("ResetMode --merge Not implemented")
	}
	if opts.Keep {
		return fmt.Errorf("ResetMode --keep Not implemented")
	}

	comm, err := cmt.CommitID(c)
	if err != nil {
		return err
	}
	if err := UpdateRef(c, UpdateRefOptions{}, "HEAD", comm, fmt.Sprintf("reset: moving to %v", comm)); err != nil {
		return err
	}
	if opts.Mixed || opts.Hard {
		if _, err := ReadTree(c, ReadTreeOptions{}, comm); err != nil {
			return err
		}
	}
	if opts.Hard {
		if err := CheckoutIndex(c, CheckoutIndexOptions{All: true, Force: true}, nil); err != nil {
			return err
		}
	}
	return nil
}

// ResetUnstage implements "git reset [<treeish>] -- paths
func ResetUnstage(c *Client, opts ResetOptions, tree Treeish, files []File) error {
	index, _ := c.GitDir.ReadIndex()
	diffs, err := DiffIndex(c, DiffIndexOptions{Cached: true}, index, tree, files)
	if err != nil {
		return err
	}
	for _, entry := range diffs {
		if err := index.AddStage(c, entry.Name, entry.Src.Sha1, Stage0, uint32(entry.SrcSize), true); err != nil {
			return err
		}
	}

	f, err := c.GitDir.Create(File("index"))
	if err != nil {
		return err
	}
	defer f.Close()
	if err := index.WriteIndex(f); err != nil {
		return err
	}

	if !opts.Quiet {
		newdiff, err := DiffFiles(c, DiffFilesOptions{}, nil)
		if err != nil {
			return err
		}
		if len(newdiff) > 0 {
			fmt.Printf("Unstaged changes after reset:\n")
		}
		for _, file := range newdiff {
			var status string
			if file.Dst == (TreeEntry{}) {
				status = "D"
			} else {
				status = "M"
			}
			fmt.Printf("%v\t%v\n", status, file.Name)
		}

	}
	return nil
}
