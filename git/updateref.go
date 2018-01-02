package git

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type UpdateRefOptions struct {
	// Not implemented
	Delete bool

	NoDeref      bool
	CreateReflog bool
	OldValue     Commitish

	// Not implemented
	Stdin         io.Reader
	NullTerminate bool
}

func updateReflog(c *Client, create bool, file File, oldvalue, newvalue Commitish, reason string) error {
	if !file.Exists() {
		if !create {
			return fmt.Errorf("Can not create new reflog for %s. --create-reflog not specified.", file)
		}
		if err := file.Create(); err != nil {
			return err
		}
	}

	now := time.Now()
	commiter := c.GetAuthor(&now)

	var toAppend string

	var oldsha, newsha CommitID
	var err error
	if oldvalue != nil {
		oldsha, err = oldvalue.CommitID(c)
		if err != nil {
			return err
		}
	}
	if newvalue != nil {
		newsha, err = newvalue.CommitID(c)
		if err != nil {
			return err
		}
	}
	if reason == "" {
		toAppend = fmt.Sprintf("%s %s %s\n", oldsha, newsha, commiter)
	} else {
		toAppend = fmt.Sprintf("%s %s %s\t%s\n", oldsha, newsha, commiter, reason)
	}
	return file.Append(toAppend)
}

// Safely updates ref to point to cmt under the client c, logging reason in the reflog.
// If opts.OldValue is set, it will return an error if the current value is not OldValue.
func UpdateRefSpec(c *Client, opts UpdateRefOptions, ref RefSpec, cmt CommitID, reason string) error {
	if opts.OldValue != nil {
		oldval, err := opts.OldValue.CommitID(c)
		if err != nil {
			return err
		}
		curval, err := ref.CommitID(c)
		if err != nil && oldval != (CommitID{}) {
			return err
		}
		if curval != oldval {
			return fmt.Errorf("%s is not equal to %s (is %s)", ref, oldval, curval)
		}
	}
	if opts.Delete {
		return fmt.Errorf("Delete RefSpec not implemented")
	}

	// The RefSpec Stringer method strips out trailing newlines and junk.
	filename := File(ref.String())
	if err := updateReflog(c, opts.CreateReflog, File(c.GitDir)+"/logs/"+filename, opts.OldValue, cmt, reason); err != nil {
		return err
	}

	file, err := c.GitDir.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	fmt.Fprintf(file, "%s\n", cmt)
	return nil
}

// Handles "git update-ref" command line. ref is what's passed on the command-line
// it can be either a symbolic ref, or a refspec. We just use a string, because
// Go doesn't support sum types.
func UpdateRef(c *Client, opts UpdateRefOptions, ref string, cmt CommitID, reason string) error {
	if opts.Stdin != nil {
		return fmt.Errorf("UpdateRef batch mode not implemented")
	}

	// It's not a symbolic ref, it's a real ref. Just directly call UpdateRefSpec
	if strings.HasPrefix(ref, "refs/") {
		return UpdateRefSpec(c, opts, RefSpec(strings.TrimSpace(ref)), cmt, reason)
	}

	// It's a symbolic ref. Dereference it, unless --deref
	if !opts.NoDeref {
		refspec, err := SymbolicRefGet(c, SymbolicRefOptions{}, SymbolicRef(ref))
		if err == DetachedHead {
			goto noderef
		}
		if err != nil {
			return err
		}
		// This is duplicated from UpdateRefSpec, but we should check
		// the value before updating the reflog. If we're not doing
		// anything, we shouldn't update the reflog. (It stays in
		// UpdateRefSpec because someone else might call it directly.)
		if opts.OldValue != nil {
			curval, err := SymbolicRef(ref).CommitID(c)
			if err != nil && opts.OldValue != (CommitID{}) {
				return err
			}
			oldval, err := opts.OldValue.CommitID(c)
			if err != nil {
				return err
			}
			if curval != oldval {
				return fmt.Errorf("%s is not equal to %s (is %s)", ref, oldval, curval)
			}
		}

		// Update the symbolic-ref reflog before doing anything. If it can't
		// be updated, it's a fatal error and we can't update the refspec.
		if err := updateReflog(c, true, File(c.GitDir)+"/logs/"+File(ref), opts.OldValue, cmt, reason); err != nil {
			return err
		}
		return UpdateRefSpec(c, opts, refspec, cmt, reason)
	}

noderef:
	// NoDeref was specified.
	if err := updateReflog(c, true, File(c.GitDir)+"/logs/"+File(ref), opts.OldValue, cmt, reason); err != nil {
		return err
	}

	f, err := c.GitDir.Create(File(ref))
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintf(f, "%s", cmt)
	return nil
}
