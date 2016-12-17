package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func getSymbolicRef(c *Client, symname string) RefSpec {
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

func updateSymbolicRef(c *Client, symname, refvalue string) RefSpec {
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

func SymbolicRef(c *Client, args []string) RefSpec {
	var startAt int
	var skipNext bool
	//	var reason string
	for idx, val := range args {
		if skipNext == true {
			skipNext = false
			continue
		}

		switch val {
		case "-m":
			//	reason = args[idx+1]
			startAt = idx + 1
		}

	}

	args = args[startAt:]
	switch len(args) {
	case 1:
		return getSymbolicRef(c, args[0])
	case 2:
		return updateSymbolicRef(c, args[0], args[1])
	default:
		panic("Arguments were parsed incorrectly or invalid. Can't get or update symbolic ref")
	}
}
