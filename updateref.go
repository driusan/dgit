package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	"os"
)

func UpdateRef(repo *libgit.Repository, args []string) {
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
			startAt = idx + 2
		}

	}

	args = args[startAt:]
	//	var oldRef string
	var newValue string
	var ref string
	switch len(args) {
	case 0, 1:
		panic("Need at least 2 arguments to update-ref")
	case 3:
		panic("Checking oldref not yet implemented")
		//	oldRef = args[2]
		fallthrough
	case 2:
		ref = SymbolicRef(repo, []string{args[0]})
		newValue = args[1]
	default:
		panic("Arguments were parsed incorrectly or invalid. Can't get or update symbolic ref")
	}
	file, err := os.Create(repo.Path + "/" + ref)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not update reference %s\n", ref)
	}
	defer file.Close()
	fmt.Fprintf(file, newValue)

}
