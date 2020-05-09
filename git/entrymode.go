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

// we sometimes see these coming from git9, although they chouldn't be valid
// according to the git spec. We need to handle them to clone some repos anyways
const modeGit9Tree = EntryMode(0040755)

// TreeType prints the entry mode as the type that shows up in the "git ls-tree"
// command.
func (e EntryMode) TreeType() string {

	switch e {
	case ModeBlob, ModeExec, ModeSymlink:
		return "blob"
	case ModeCommit:
		return "commit"
	case ModeTree, modeGit9Tree:
		return "tree"
	default:
		panic(fmt.Sprintf("Invalid mode %o", e))
	}
}

func ModeFromString(s string) (EntryMode, error) {
	switch s {
	case "100644":
		return ModeBlob, nil
	case "100755":
		return ModeExec, nil
	case "120000":
		return ModeSymlink, nil
	case "160000":
		return ModeCommit, nil
	case "040755":
		return modeGit9Tree, fmt.Errorf("Bad tree type %v", s)
	case "040000":
		return ModeTree, nil
	default:
		return 0, fmt.Errorf("Unknown mode: %v", s)
	}
}
