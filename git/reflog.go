package git

import (
	"fmt"
	"os"
	"path/filepath"
)

type ReflogDeleteOptions struct{}

type ReflogExpireOptions struct {
	ReflogDeleteOptions

	Expire string
	All    bool
}

// Returns true if a reflog exists for refname r under client.
func ReflogExists(c *Client, r Refname) bool {
	path := filepath.Join(c.GitDir.String(), "logs", string(r))
	return c.GitDir.File(File(path)).Exists()
}

func ReflogExpire(c *Client, opts ReflogExpireOptions, refpatterns []string) error {
	if opts.Expire != "now" || len(refpatterns) != 0 {
		return fmt.Errorf("Only reflog --expire=now --all currently supported")
	}
	if opts.All && len(refpatterns) != 0 {
		return fmt.Errorf("Can not combine --all with explicit refs")
	}

	// If expire is now, we just truncate applicable reflogs
	if opts.Expire == "now" && opts.All {
		// This is a hack to get fsck's test setup working. Since
		// we're expiring everything, we just truncate everything
		// in the .git/logs directory. (We can't delete them, because
		// the reflogs need to still exist.)
		//
		// (There's a catch-22 where the fsck tests depend on reflog,
		// and the reflog tests depend on fsck.)
		filepath.Walk(filepath.Join(c.GitDir.String(), "logs"),
			func(path string, info os.FileInfo, err error) error {
				if info.IsDir() {
					return nil
				}

				// Use Create to truncate the file. We know it exists
				// because we got here from a Walk to it, so we don't
				// worry about creating new files.
				fi, err := os.Create(path)
				if err != nil {
					return err
				}
				if err := fi.Close(); err != nil {
					return err
				}
				return nil
			})
	}
	return nil
}
