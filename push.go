package main

import (
	"fmt"
	"os"
	"strings"

	libgit "github.com/driusan/git"
)

func Push(c *Client, repo *libgit.Repository, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Missing repository to fetch")
		return
	}

	file, err := c.GitDir.Open("config")
	if err != nil {
		panic("Couldn't open config\n")
	}
	defer file.Close()
	config := parseConfig(repo, file)
	remote := config.GetConfig("branch." + args[0] + ".remote")
	mergebranch := strings.TrimSpace(config.GetConfig("branch." + args[0] + ".merge"))
	repoid := config.GetConfig("remote." + remote + ".url")
	println(remote, " on ", repoid)
	var ups uploadpack
	if repoid[0:7] == "http://" || repoid[0:8] == "https://" {
		ups = &smartHTTPServerRetriever{location: repoid,
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
		trimmed := strings.TrimSuffix(ref.Refname, "\000")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed == mergebranch {
			localSha, err := RevParse(c, repo, []string{args[0]})
			if err != nil {
				panic(err)
			}
			fmt.Printf("Refname: %s Remote Sha1: %s Local Sha1: %s\n", ref.Refname, ref.Sha1, localSha[0].Id)
			objects, err := RevList(c, repo, []string{"--objects", "--quiet", localSha[0].Id.String(), "^" + ref.Sha1})
			if err != nil {
				panic(err)
			}
			var lines string
			for _, o := range objects {
				lines = fmt.Sprintf("%s%s\n", lines, o.String())
			}

			PackObjects(repo, strings.NewReader(lines), []string{"foo"})
			f, err := os.Open("foo.pack")
			if err != nil {
				panic(err)
			}
			defer f.Close()
			stat, err := f.Stat()
			if err != nil {
				panic(err)
			}
			print(stat.Size())
			//defer os.Remove("foo.pack")
			ups.SendPack(UpdateReference{
				LocalSha1:  localSha[0].Id.String(),
				RemoteSha1: ref.Sha1,
				Refname:    ref.Refname,
			}, f, stat.Size())
		}
	}

}
