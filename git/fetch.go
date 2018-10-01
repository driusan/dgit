package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

type FetchOptions struct {
	FetchPackOptions
}

func Fetch(c *Client, opts FetchOptions, rmt Remote) error {
	opts.FetchPackOptions.All = true
	opts.FetchPackOptions.Verbose = true
	newrefs, err := FetchPack(c, opts.FetchPackOptions, rmt, nil)
	if err != nil {
		return err
	}
	for _, ref := range newrefs {
		if c.GitDir != "" {
			if strings.HasPrefix(ref.Name, "refs/heads") {
				// FIXME: This should use update ref and also
				// print a better message.
				os.MkdirAll(c.GitDir.File(File("refs/remotes/"+rmt.String())).String(), 0755)
				refname := strings.Replace(ref.Name, "refs/heads/", "refs/remotes/"+rmt.String()+"/", 1)
				refloc := fmt.Sprintf("%s/%s", c.GitDir, refname)
				fmt.Printf("Creating %s with %s\n", refloc, ref.Value)
				ioutil.WriteFile(
					refloc,
					[]byte(ref.Value.String() + "\n"),
					0644,
				)
			}
		}
	}

	return nil
}
