package cmd

import (
	"fmt"
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

	c, err := git.Init(c, git.InitOptions{Quiet: true}, dirName)
	if err != nil {
		return err
	}

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

	cmtish, err := git.RevParseCommitish(c, &git.RevParseOptions{}, "origin/master")
	if err != nil {
		return err
	}
	cmt, err := cmtish.CommitID(c)
	if err != nil {
		return err
	}

	// Update the master branch to point to the same commit as origin/master
	if err := git.UpdateRefSpec(
		c,
		git.UpdateRefOptions{CreateReflog: true, OldValue: git.CommitID{}},
		git.RefSpec("refs/heads/master"),
		cmt,
		"clone: "+args[0],
	); err != nil {
		return err
	}

	// HEAD is already pointing to refs/heads/master from init, but the logs/HEAD
	// reflog isn't created yet. We can cheat by just copying the one created for
	// the master branch by UpdateRefSpec above.
	reflog, err := c.GitDir.ReadFile("logs/refs/heads/master")
	if err != nil {
		return err
	}

	if err := c.GitDir.WriteFile("logs/HEAD", reflog, 0755); err != nil {
		return err
	}

	// Since this is an initial clone, we just do a hard reset and don't
	// try and be intelligent about what we're checking out.
	Reset(c, []string{"--hard"})
	return nil
}
