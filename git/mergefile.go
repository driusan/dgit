package git

import (
	"bytes"
	"io"
	"os"
	"os/exec"
)

type MergeFileFile struct {
	Filename File
	Label    string
}

type MergeFileOptions struct {
	Current, Base, Other MergeFileFile

	Quiet  bool
	Stdout bool
	Diff3  bool
}

// MergeFile merges changes that lead from opt.Base to opt.Other into opt.Current,
// flagging conflictsas appropriate.
//
// This will return an io.Reader of the merged state rather than directly Current.
//
// BUG(driusan): this currently invokes an external diff3 tool, which needs to be
// installed in order to work.
func MergeFile(c *Client, opt MergeFileOptions) (io.Reader, error) {
	var args []string
	if opt.Current.Label != "" {
		args = append(args, "-L", opt.Current.Label)
	} else {
		// Ensure there's the right amount of -L specified if base or other happen
		// to have specified a label but current didn't.
		args = append(args, "-L", opt.Current.Label)
	}

	if opt.Base.Label != "" {
		args = append(args, "-L", opt.Base.Label)
	} else {
		args = append(args, "-L", opt.Base.Label)
	}

	if opt.Other.Label != "" {
		args = append(args, "-L", opt.Other.Label)
	} else {
		// Ensure there's the right amount of -L specified if base or other happen
		// to have specified a label but current didn't.
		args = append(args, "-L", opt.Other.Label)
	}

	// FIXME: There should be a way to get output similar to -A for -X, but
	// -X doesn't seem to include non-conflicting lines. So for now, this always
	// uses diff3 output.
	args = append(args, "-m", "-A")
	args = append(args, opt.Current.Filename.String(), opt.Base.Filename.String(), opt.Other.Filename.String())
	var output bytes.Buffer
	diff3cmd := exec.Command(posixDiff3, args...)
	diff3cmd.Stderr = os.Stderr
	diff3cmd.Stdout = &output
	err := diff3cmd.Run()
	return &output, err
}
