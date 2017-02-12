package git

import (
	"fmt"
	"os"
	"os/exec"
)

// Will invoke the Client's editor to edit the file f.
func (c *Client) ExecEditor(f File) error {
	editor := os.Getenv("editor")
	if editor == "" {
		fmt.Fprintf(os.Stderr, "Warning: editor environment not set. Falling back on ed...\n")
		editor = "ed"
	}

	cmd := exec.Command(editor, f.String())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
