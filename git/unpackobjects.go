package git

import (
	"io"
)

type UnpackObjectsOptions struct {
	// Do not write any objects
	DryRun bool

	// Do not print any progress information to os.Stderr
	Quiet bool

	// Attempt to recover corrupt pack files (not implemented)
	Recover bool

	// Do not write objects with broken content or links (not implemented)
	Strict bool

	// Do not attempt to process packfiles larger than this size.
	// (the value "0" means unlimited.)
	MaxInputSize uint
}

// Unpack the objects from r's input stream into the client GitDir's
// objects directory.
func UnpackObjects(c *Client, opts UnpackObjectsOptions, r io.ReadSeeker) error {
	_, err := Unpack(c, opts, r)
	return err
}
