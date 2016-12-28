package git

import (
	"strings"
)

// A RefSpec refers to a reference contained under .git/refs
type RefSpec string

func (r RefSpec) String() string {
	if len(r) < 1 {
		return ""
	}

	// This will only trim a single nil byte, but if there's more
	// than that we're doing something really wrong.
	return strings.TrimSpace(strings.TrimSuffix(string(r), "\000"))
}

// Returns the file that holds r.
func (r RefSpec) File(c *Client) File {
	return c.GitDir.File(File(r.String()))
}

// Returns the value of RefSpec in Client's GitDir, or the empty string
// if it doesn't exist.
func (r RefSpec) Value(c *Client) (string, error) {
	f := r.File(c)
	val, err := f.ReadAll()
	return strings.TrimSpace(val), err
}
