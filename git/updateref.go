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
	OldValue     string

	// Not implemented
	Stdin         io.Reader
	NullTerminate bool
}

func updateReflog(c *Client, create bool, file File, oldvalue, newvalue string, reason string) error {
	if len(oldvalue) != 40 || len(newvalue) != 40 {
		return fmt.Errorf("Invalid commit value. Must be 40 character hex.")
	}
	if !create && !file.Exists() {
		return fmt.Errorf("Can not create new reflog for %s. --create-reflog not specified.", file)
	}

	now := time.Now()
	commiter := c.GetAuthor(&now)

	var toAppend string
	if reason == "" {
		toAppend = fmt.Sprintf("%s %s %s\n", oldvalue, newvalue, commiter)
	} else {
		toAppend = fmt.Sprintf("%s %s %s\t%s\n", oldvalue, newvalue, commiter, reason)
	}
	return file.Append(toAppend)
}

// Safely updates ref to point to cmt under the client c, logging reason in the reflog.
// If opts.OldValue is set, it will return an error if the current value is not OldValue.
func UpdateRefSpec(c *Client, opts UpdateRefOptions, ref RefSpec, cmt CommitID, reason string) error {
	if opts.OldValue != "" {
		val, err := ref.Value(c)
		if err != nil {
			return err
		}
		if val != opts.OldValue {
			return fmt.Errorf("%s is not equal to %s (is %s)", ref, opts.OldValue, val)
		}
	}
	if opts.Delete {
		return fmt.Errorf("Delete RefSpec not implemented")
	}

	// The RefSpec Stringer method strips out trailing newlines and junk.
	filename := File(ref.String())
	print("filename, ", filename)
	if err := updateReflog(c, opts.CreateReflog, File(c.GitDir)+"/logs/"+filename, opts.OldValue, cmt.String(), reason); err != nil {
		return err
	}

	file, err := c.GitDir.Create(filename)
	fmt.Printf("%s", file)
	if err != nil {
		return err
	}
	defer file.Close()
	fmt.Fprintf(file, "%s\n", cmt)
	return nil
}

// Handles "git update-ref" command line. If ref is what's passed on the command-line
// it can be either a symbolic ref, or a refspec.
func UpdateRef(c *Client, opts UpdateRefOptions, ref string, cmt CommitID, reason string) error {
	if opts.Stdin != nil {
		return fmt.Errorf("UpdateRef batch mode not implemented")
	}

	if strings.HasPrefix(ref, "refs/") {
		return UpdateRefSpec(c, opts, RefSpec(strings.TrimSpace(ref)), cmt, reason)
	}
	if refspec, err := SymbolicRefGet(c, SymbolicRefOptions{}, ref); !opts.NoDeref && refspec != "" && err == nil {
		// This is duplicated from UpdateRefSpec, but we should check
		// the value before updating the reflog. If we're not doing
		// anything, we shouldn't update the reflog. (It stays in
		// UpdateRefSpec because someone else might call it directly.)
		if opts.OldValue != "" {
			val, err := refspec.Value(c)
			if err != nil {
				return err
			}
			if val != opts.OldValue {
				return fmt.Errorf("%s is not equal to %s (is %s)", ref, opts.OldValue, val)
			}
		}

		// Update the symbolic-ref reflog before doing anything. If it can't
		// be updated, it's a fatal error and we can't update the refspec.
		if err := updateReflog(c, true, File(c.GitDir)+"/logs/"+File(ref), opts.OldValue, cmt.String(), reason); err != nil {
			return err
		}
		return UpdateRefSpec(c, opts, refspec, cmt, reason)
	}
	return fmt.Errorf("Invalid reference %s", ref)
}
