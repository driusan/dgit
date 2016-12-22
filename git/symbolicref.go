package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func SymbolicRefGet(c *Client, symname string) RefSpec {
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

func SymbolicRefUpdate(c *Client, reflogmessage string, symname string, refvalue RefSpec) RefSpec {
	if len(refvalue) < 5 || refvalue[0:5] != "refs/" {
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
