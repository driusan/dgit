package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/driusan/dgit/git"
)

func Fetch(c *git.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Missing repository to fetch")
		return
	}

	file, err := c.GitDir.Open("config")
	if err != nil {
		panic("Couldn't open config\n")
	}
	defer file.Close()
	config := git.ParseConfig(file)
	repoid := config.GetConfig("remote." + args[0] + ".url")
	var ups git.Uploadpack
	if repoid[0:7] == "http://" || repoid[0:8] == "https://" {
		ups = &git.SmartHTTPServerRetriever{Location: repoid,
			C: c,
		}
	} else {
		fmt.Fprintln(os.Stderr, "Unknown protocol.")
		return
	}
	refs, pack, err := ups.NegotiatePack()
	switch err {
	case git.NoNewCommits:
		return
	case nil:
		break
	default:
		panic(err)
	}
	defer pack.Close()
	defer os.RemoveAll(pack.Name())
	pack.Seek(0, 0)
	git.UnpackObjects(c, git.UnpackObjectsOptions{}, pack)
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
