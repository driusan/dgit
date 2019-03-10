package git

import (
	"path/filepath"
)

// Returns true if a reflog exists for refname r under client.
func ReflogExists(c *Client, r Refname) bool {
	path := filepath.Join(c.GitDir.String(), "logs", string(r))
	return c.GitDir.File(File(path)).Exists()
}
