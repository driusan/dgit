package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	"io/ioutil"
	"os"
)

func Fetch(c *Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Missing repository to fetch")
		return
	}

	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return
	}

	file, err := c.GitDir.Open("config")
	if err != nil {
		panic("Couldn't open config\n")
	}
	defer file.Close()
	config := parseConfig(file)
	repoid := config.GetConfig("remote." + args[0] + ".url")
	var ups uploadpack
	if repoid[0:7] == "http://" || repoid[0:8] == "https://" {
		ups = &smartHTTPServerRetriever{location: repoid,
			c: c,
		}
	} else {
		fmt.Fprintln(os.Stderr, "Unknown protocol.")
		return
	}
	refs, pack, err := ups.NegotiatePack()
	switch err {
	case NoNewCommits:
		return
	case nil:
		break
	default:
		panic(err)
	}
	defer pack.Close()
	defer os.RemoveAll(pack.Name())
	pack.Seek(0, 0)
	fmt.Printf("Unpacking into %s\n", c.GitDir)
	unpack(c, repo, pack)
	for _, ref := range refs {
		if c.GitDir != "" {
			refloc := fmt.Sprintf("%s/%s", c.GitDir, ref.Refname.String())
			fmt.Printf("Creating %s with %s", refloc, ref.Sha1)
			ioutil.WriteFile(
				refloc,
				[]byte(ref.Sha1),
				0644,
			)
		}
	}
}
