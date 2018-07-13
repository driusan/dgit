package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

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
	repoid, _ := config.GetConfig("remote." + args[0] + ".url")
	var ups git.Uploadpack
	if strings.HasPrefix(repoid, "http://") || strings.HasPrefix(repoid, "https://") {
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
	if pack != nil {
		defer pack.Close()
	}
	_, err = git.IndexAndCopyPack(c, git.IndexPackOptions{Verbose: true}, pack)
	if err != nil {
		panic(err)
	}
	for _, ref := range refs {
		if c.GitDir != "" {
			refname := ref.Refname.String()
			if strings.HasPrefix(refname, "refs/heads") {
				os.MkdirAll(c.GitDir.File(git.File("refs/remotes/"+args[0])).String(), 0755)
				refname = strings.Replace(refname, "refs/heads/", "refs/remotes/"+args[0]+"/", 1)
				refloc := fmt.Sprintf("%s/%s", c.GitDir, refname)
				fmt.Printf("Creating %s with %s", refloc, ref.Sha1)
				ioutil.WriteFile(
					refloc,
					[]byte(ref.Sha1),
					0644,
				)
			}

		}
	}
}
