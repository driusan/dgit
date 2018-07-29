package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

type FetchOptions struct {
}

func Fetch(c *Client, opts FetchOptions, repository string) error {
	config, err := LoadLocalConfig(c)
	if err != nil {
		panic(err)
	}

	repoid, _ := config.GetConfig("remote." + repository + ".url")

	var ups Uploadpack
	if strings.HasPrefix(repoid, "http://") || strings.HasPrefix(repoid, "https://") {
		ups = &SmartHTTPServerRetriever{Location: repoid,
			C: c,
		}
	} else {
		return fmt.Errorf("Unknown protocol %s", repoid)
	}

	refs, pack, err := ups.NegotiatePack()
	switch err {
	case NoNewCommits:
		return nil
	case nil:
		break
	default:
		panic(err)
	}
	if pack != nil {
		defer pack.Close()
	}
	_, err = IndexAndCopyPack(c, IndexPackOptions{Verbose: true}, pack)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if c.GitDir != "" {
			refname := ref.Refname.String()
			if strings.HasPrefix(refname, "refs/heads") {
				os.MkdirAll(c.GitDir.File(File("refs/remotes/"+repository)).String(), 0755)
				refname = strings.Replace(refname, "refs/heads/", "refs/remotes/"+repository+"/", 1)
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

	return nil
}
