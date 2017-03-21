package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

func Clone(c *git.Client, args []string) error {
	var repoid string
	var dirName string
	// TODO: This argument parsing should be smarter and more
	// in line with how cgit does it.
	switch len(args) {
	case 0:
		return fmt.Errorf("Invalid usage")
	case 1:
		repoid = args[0]
	case 2:
		repoid = args[0]
		dirName = args[1]
	default:
		repoid = args[0]
	}
	repoid = strings.TrimRight(repoid, "/")
	pieces := strings.Split(repoid, "/")

	if dirName == "" {
		if len(pieces) > 0 {
			dirName = pieces[len(pieces)-1]
		}
		dirName = strings.TrimSuffix(dirName, ".git")

		if _, err := os.Stat(dirName); err == nil {
			return fmt.Errorf("Directory %s already exists, can not clone.\n", dirName)
		}
		if dirName == "" {
			panic("No directory left to clone into.")
		}
	}

	c = Init(c, []string{dirName})

	Config(c, []string{"--set", "remote.origin.url", repoid})
	Config(c, []string{"--set", "branch.master.remote", "origin"})

	// This should be smarter and try and get the HEAD branch from Fetch.
	// The HEAD refspec isn't necessarily named refs/heads/master.
	Config(c, []string{"--set", "branch.master.merge", "refs/heads/master"})

	Fetch(c, []string{"origin"})

	// Create an empty reflog for HEAD, since this is an initial clone, and then
	// point HEAD at refs/heads/master
	if err := os.MkdirAll(c.GitDir.File("logs").String(), 0755); err != nil {
		return err
	}
	if err := ioutil.WriteFile(c.GitDir.File("logs/HEAD").String(), []byte{}, 0644); err != nil {
		return err
	}
	// The references were unpacked into refs/remotes/origin/, there's
	// still no master branch set up, so copy refs/remotes/origin/master
	// into refs/heads/master before doing a reset.
	remoteMaster, err := ioutil.ReadFile(c.GitDir.File("refs/remotes/origin/master").String())
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(c.GitDir.File("refs/heads/master").String(), remoteMaster, 0644); err != nil {
		return err
	}

	// This needs to be done after writing the above, or updateRefLog will
	// complain that refs/heads/master doesn't exist.
	if err := git.SymbolicRefUpdate(c, git.SymbolicRefOptions{}, "HEAD", "refs/heads/master", "clone: "+args[0]); err != nil {
		return err
	}

	// Since this is an initial clone, we just do a hard reset and don't
	// try and be intelligent about what we're checking out.
	Reset(c, []string{"--hard"})
	return nil
}
