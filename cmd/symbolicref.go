package cmd

import (
	"github.com/driusan/go-git/git"
)

func SymbolicRef(c *git.Client, args []string) git.RefSpec {
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
		return git.SymbolicRefGet(c, args[0])
	case 2:
		return git.SymbolicRefUpdate(c, "", args[0], git.RefSpec(args[1]))
	default:
		panic("Arguments were parsed incorrectly or invalid. Can't get or update symbolic ref")
	}
}
