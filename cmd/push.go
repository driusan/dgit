package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

func Push(c *git.Client, args []string) {
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
	remote := config.GetConfig("branch." + args[0] + ".remote")
	mergebranch := strings.TrimSpace(config.GetConfig("branch." + args[0] + ".merge"))
	repoid := config.GetConfig("remote." + remote + ".url")
	println(remote, " on ", repoid)
	var ups git.Uploadpack
	if repoid[0:7] == "http://" || repoid[0:8] == "https://" {
		ups = &git.SmartHTTPServerRetriever{Location: repoid,
			C: c,
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
		trimmed := ref.Refname.String()
		if trimmed == mergebranch {
			localSha, err := RevParse(c, []string{args[0]})
			if err != nil {
				panic(err)
			}
			fmt.Printf("Refname: %s Remote Sha1: %s Local Sha1: %s\n", ref.Refname, ref.Sha1, localSha[0].Id)
			objects, err := RevList(c, []string{"--objects", "--quiet", localSha[0].Id.String(), "^" + ref.Sha1})
			if err != nil {
				panic(err)
			}
			var lines string
			for _, o := range objects {
				lines = fmt.Sprintf("%s%s\n", lines, o.String())
			}

			f, err := ioutil.TempFile("", "sendpack")
			if err != nil {
				panic(err)
			}
			defer os.Remove(f.Name())

			PackObjects(c, strings.NewReader(lines), []string{f.Name()})
			f, err = os.Open(f.Name() + ".pack")
			if err != nil {
				panic(err)
			}
			defer os.Remove(f.Name())

			stat, err := f.Stat()
			if err != nil {
				panic(err)
			}
			ups.SendPack(git.UpdateReference{
				LocalSha1:  localSha[0].Id.String(),
				RemoteSha1: ref.Sha1,
				Refname:    ref.Refname,
			}, f, stat.Size())
		}
	}

}
