package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	"io/ioutil"
	"os"
	"strings"
)

func getSymbolicRef(repo *libgit.Repository, symname string) string {
	file, err := os.Open(repo.Path + "/" + symname)
	if err != nil {
		return ""
	}
	value, err := ioutil.ReadAll(file)
	if err != nil {
		return ""
	}

	if prefix := string(value[0:5]); prefix != "ref: " {
		return ""
	}
	return strings.TrimSpace(string(value[5:]))

}

func updateSymbolicRef(repo *libgit.Repository, symname, refvalue string) string {
	if len(refvalue) < 5 || refvalue[0:5] != "refs/" {
		fmt.Fprintf(os.Stderr, "fatal: Refusing to point "+symname+" outside of refs/")
		return ""
	}
	file, err := os.Create(repo.Path + "/" + symname)
	if err != nil {
		return ""
	}
	fmt.Fprintf(file, "ref: %s", refvalue)
	return ""
}
func SymbolicRef(repo *libgit.Repository, args []string) string {
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
		return getSymbolicRef(repo, args[0])
	case 2:
		return updateSymbolicRef(repo, args[0], args[1])
	default:
		panic("Arguments were parsed incorrectly or invalid. Can't get or update symbolic ref")
	}
}
