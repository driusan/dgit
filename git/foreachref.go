package git

import (
	"os"
	"path/filepath"
	"strings"
)

// Calls callback for each ref under c's GitDir which has prefix as a prefix.
func ForEachRefCallback(c *Client, prefix string, callback func(*Client, Ref) error) error {
	// FIXME: Include packed refs.
	err := filepath.Walk(
		c.GitDir.File("refs").String(),
		func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			refname := strings.TrimPrefix(path, c.GitDir.String()+"/")
			if strings.HasPrefix(refname, prefix) {
				r, err := parseRef(c, refname)
				if err != nil {
					return err
				}
				if err := callback(c, r); err != nil {
					return err
				}
			}
			return nil
		},
	)
	return err
}
