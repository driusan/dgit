package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// SymbolicRefOptions represents the command line options
// that may be passed on the command line. (NB. None of these
// are implemented.)
type SymbolicRefOptions struct {
	Quiet  bool
	Delete bool
	Short  bool
}

// Gets a RefSpec for a symbolic ref. Returns "" if symname is not a valid
// symbolic ref.
func SymbolicRefGet(c *Client, opts SymbolicRefOptions, symname string) RefSpec {
	file, err := c.GitDir.Open(File(symname))
	if err != nil {
		return ""
	}
	defer file.Close()
	value, err := ioutil.ReadAll(file)
	if err != nil {
		return ""
	}

	if prefix := string(value[0:5]); prefix != "ref: " {
		return ""
	}
	return RefSpec(strings.TrimSpace(string(value[5:])))

}

func SymbolicRefUpdate(c *Client, opts SymbolicRefOptions, symname string, refvalue RefSpec, reason string) RefSpec {
	if !strings.HasPrefix(refvalue.String(), "refs/") {
		fmt.Fprintf(os.Stderr, "fatal: Refusing to point "+symname+" outside of refs/")
		return ""
	}
	file, err := c.GitDir.Create(File(symname))
	if err != nil {
		return ""
	}
	defer file.Close()
	fmt.Fprintf(file, "ref: %s", refvalue)
	return ""
}
