package git

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
