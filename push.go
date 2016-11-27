package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	"os"
)

func Push(repo *libgit.Repository, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Missing repository to fetch")
		return
	}

	file, err := os.Open(repo.Path + "/config")
	if err != nil {
		panic("Couldn't open config\n")
	}
	defer file.Close()
	config := parseConfig(repo, file)
	repoid := config.GetConfig("remote." + args[0] + ".url")
	var ups uploadpack
	if repoid[0:7] == "http://" || repoid[0:8] == "https://" {
		ups = smartHTTPServerRetriever{location: repoid,
			repo: repo,
		}
	} else {
		fmt.Fprintln(os.Stderr, "Unknown protocol.")
		return
	}
	refs, err := ups.NegotiateSendPack()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		return
	}
	for _, ref := range refs {
		fmt.Printf("Refname: %s Sha1: %s\n", ref.Refname, ref.Sha1)
	}

}
