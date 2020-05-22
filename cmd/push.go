package cmd

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

func Push(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("branch", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"all", "mirror", "tags", "follow-tags", "atomic", "n", "dry-run", "f", "force", "delete", "prune", "v", "verbose", "u", "no-signed", "no-verify"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"receive-pack", "repo", "o", "push-option", "signed", "force-with-lease"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	setupstream := flags.String("set-upstream", "", "Sets the upstream remote for the branch")

	flags.Parse(args)

	if flags.NArg() < 1 {
		fmt.Fprintf(flag.CommandLine.Output(), "Missing repository to push")
		flag.Usage()
		os.Exit(2)
	} else if flags.NArg() > 1 {
		fmt.Fprintf(flag.CommandLine.Output(), "Providing a refspec is not currently implemented")
		flag.Usage()
		os.Exit(2)
	}

	bname := flags.Arg(0)
	config, err := git.LoadLocalConfig(c)
	if err != nil {
		return err
	}
	if *setupstream != "" {
		config.SetConfig(fmt.Sprintf("branch.%v.remote", bname), *setupstream)
		config.SetConfig(fmt.Sprintf("branch.%v.merge", bname), fmt.Sprintf("refs/heads/%v", bname))
		if err := config.WriteConfig(); err != nil {
			return err
		}
	}
	remote, _ := config.GetConfig("branch." + bname + ".remote")
	if remote == "" {
		return fmt.Errorf(`The branch %v has no upstream set.
To push and set the upstream to the remote named "origin" use:

	%v push --set-upstream origin %v

`, bname, os.Args[0], bname)

	}
	mergebranch, _ := config.GetConfig("branch." + bname + ".merge")
	mergebranch = strings.TrimSpace(mergebranch)
	repoid, _ := config.GetConfig("remote." + remote + ".url")
	println(remote, " on ", repoid)
	var ups git.Uploadpack
	if repoid[0:7] == "http://" || repoid[0:8] == "https://" {
		ups = &git.SmartHTTPServerRetriever{Location: repoid,
			C: c,
		}
	} else {
		return fmt.Errorf("Unknown protocol.")
	}

	refs, err := ups.NegotiateSendPack()
	if err != nil {
		return err
	}

	localSha, _, err := RevParse(c, []string{flags.Arg(0)})
	if err != nil {
		return err
	}
	var remoteCommits []git.Commitish
	var remoteHead git.CommitID
	for _, ref := range refs {
		trimmed := ref.Refname.String()
		refsha, err := git.Sha1FromString(ref.Sha1)
		if err != nil {
			return err
		}
		if trimmed == mergebranch {
			remoteHead = git.CommitID(refsha)
		}
		if have, _, err := c.HaveObject(refsha); have && err == nil {
			remoteCommits = append(remoteCommits, git.CommitID(refsha))
		}
	}
	objs, err := git.RevList(c, git.RevListOptions{Objects: true, Quiet: true}, nil, []git.Commitish{localSha[0]}, remoteCommits)
	if err != nil {
		return err
	}

	f, err := ioutil.TempFile("", "dgitpush")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := git.PackObjects(c, git.PackObjectsOptions{}, f, objs); err != nil {
		return err
	}
	f.Seek(0, io.SeekStart)
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	ups.SendPack(git.UpdateReference{
		LocalSha1:  localSha[0].Id.String(),
		RemoteSha1: remoteHead.String(),
		Refname:    git.RefSpec(mergebranch),
	}, f, stat.Size())

	// We don't do anything special for setupstream here, because it was saved above
	if rmtname := c.GetConfig(fmt.Sprintf("branch.%v.remote", bname)); rmtname != "" {
		rmtref := git.RefSpec(fmt.Sprintf("refs/remotes/%v/%v", rmtname, bname))
		if git.UpdateRefSpec(c, git.UpdateRefOptions{}, rmtref, git.CommitID(localSha[0].Id), "update by push"); err != nil {
			return err
		}
	}
	return nil
}
