package cmd

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"../git"
)

func Push(c *git.Client, args []string) {
	flags := flag.NewFlagSet("branch", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"all", "mirror", "tags", "follow-tags", "atomic", "n", "dry-run", "f", "force", "delete", "prune", "v", "verbose", "u", "set-upstream", "no-signed", "no-verify"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"receive-pack", "repo", "o", "push-option", "signed", "force-with-lease"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	flags.Parse(args)

	if flags.NArg() < 1 {
		fmt.Fprintf(flag.CommandLine.Output(), "Missing repository to fetch")
		flag.Usage()
		os.Exit(2)
	} else if flags.NArg() > 1 {
		fmt.Fprintf(flag.CommandLine.Output(), "Providing a refspect is not currently implemented")
		flag.Usage()
		os.Exit(2)
	}

	file, err := c.GitDir.Open("config")
	if err != nil {
		panic("Couldn't open config\n")
	}
	defer file.Close()
	config := git.ParseConfig(file)
	remote, _ := config.GetConfig("branch." + flags.Arg(0) + ".remote")
	mergebranch, _ := config.GetConfig("branch." + flags.Arg(0) + ".merge")
	mergebranch = strings.TrimSpace(mergebranch)
	repoid, _ := config.GetConfig("remote." + remote + ".url")
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
			localSha, err := RevParse(c, []string{flags.Arg(0)})
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
