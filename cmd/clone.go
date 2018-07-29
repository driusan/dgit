package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

func Clone(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("clone", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.CloneOptions{}

	flag.BoolVar(&opts.InitOptions.Quiet, "quiet", false, "Operate quietly")
	flag.BoolVar(&opts.InitOptions.Quiet, "q", false, "Alias for --quiet")
	flag.BoolVar(&opts.InitOptions.Bare, "bare", false, "Make a bare Git repository.")
	template := ""
	flag.StringVar(&template, "template", "", "Specify the directory from which templates will be used.")

	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"l", "s", "no-hardlinks", "n", "mirror", "dissociate", "single-branch", "no-single-branch", "no-tags", "shallow-submodules", "no-shallow-submodules"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"template", "o", "b", "u", "reference", "separate-git-dir", "depth", "recurse-submodules", "jobs"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	flags.Parse(args)

	if template != "" {
		opts.InitOptions.Template = git.File(template)
	}

	var repoid string
	var dirName string
	// TODO: This argument parsing should be smarter and more
	// in line with how cgit does it.
	switch flags.NArg() {
	case 0:
		flags.Usage()
		os.Exit(2)
	case 1:
		repoid = flags.Arg(0)
	case 2:
		repoid = flags.Arg(0)
		dirName = flags.Arg(1)
	default:
		repoid = flags.Arg(0)
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

	c, err := git.Init(c, opts.InitOptions, dirName)
	if err != nil {
		return err
	}

	config, err := git.LoadLocalConfig(c)
	if err != nil {
		return err
	}

	config.SetConfig("remote.origin.url", repoid)
	config.SetConfig("branch.master.remote", "origin")
	// This should be smarter and try and get the HEAD branch from Fetch.
	// The HEAD refspec isn't necessarily named refs/heads/master.
	config.SetConfig("branch.master.merge", "refs/heads/master")

	err = config.WriteConfig()
	if err != nil {
		return err
	}

	err = git.FetchRepository(c, git.FetchOptions{}, "origin")
	if err != nil {
		return err
	}

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
		"clone: "+flags.Arg(0),
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
	return git.Reset(c, git.ResetOptions{Hard: true}, []git.File{})
}
