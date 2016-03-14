package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	"os"
	"strings"
)

func RevParse(repo *libgit.Repository, args []string) {
	for _, arg := range args {
		switch arg {
		case "--git-dir":
			wd, err := os.Getwd()
			if err == nil {
				fmt.Printf("%s\n", strings.TrimPrefix(repo.Path, wd + "/"))
			} else {
				fmt.Printf("%s\n", repo.Path)
			}
		default:
			if len(arg) > 0 && arg[0] == '-' {
				fmt.Printf("%s\n", arg)
			}
		}

	}

}
