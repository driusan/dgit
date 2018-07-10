package git

import (
	"fmt"
)

// An EntryMode is like an os.FileMode, but restricted to the values
// that are legal in git.
type EntryMode uint32

const (
	ModeBlob    = EntryMode(0100644)
	ModeExec    = EntryMode(0100755)
	ModeSymlink = EntryMode(0120000)
	ModeCommit  = EntryMode(0160000)
	ModeTree    = EntryMode(0040000)
)

// TreeType prints the entry mode as the type that shows up in the "git ls-tree"
// command.
func (e EntryMode) TreeType() string {

	switch e {
	case ModeBlob, ModeExec, ModeSymlink:
		return "blob"
	case ModeCommit:
		return "commit"
	case ModeTree:
		return "tree"
	default:
		panic(fmt.Sprintf("Invalid mode %o", e))
	}
}
