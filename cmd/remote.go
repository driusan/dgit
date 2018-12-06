package cmd

import (
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func printRemotes(c *git.Client, opts git.RemoteOptions) error {
	remotes, err := git.RemoteList(c, opts)
	if err != nil {
		return err
	}

	for _, r := range remotes {
		fmt.Println(r.Name())
	}
	return nil
}

func Remote(c *git.Client, args []string) error {
	flags := newFlagSet("remote")
	opts := git.RemoteOptions{}
	flags.BoolVar(&opts.Verbose, "v", false, "Make more verbose")
	flags.Parse(args)
	args = flags.Args()
	if len(args) < 1 {
		return printRemotes(c, opts)
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			return fmt.Errorf("Must provide name and URL for remote to add")
		}
		aopts := git.RemoteAddOptions{opts}
		return git.RemoteAdd(c, aopts, args[1], args[2])
	case "get-url":
		uflags := newFlagSet("remote-get-url")
		urlopts := git.RemoteGetURLOptions{RemoteOptions: opts}
		uflags.BoolVar(&urlopts.Push, "push", false, "Print push URLs, not fetch URLs")
		uflags.BoolVar(&urlopts.All, "all", false, "Print all URLs, not just the first (not implemented)")
		uflags.Parse(args[1:])
		args = uflags.Args()
		if len(args) < 1 {
			uflags.Usage()
			os.Exit(1)
		}
		urls, err := git.RemoteGetURL(c, urlopts, git.Remote(args[0]))
		if err != nil {
			return err
		}
		for _, u := range urls {
			fmt.Println(u)
		}
		return nil
	case "show":
		sflags := newFlagSet("remote-show")
		sopts := git.RemoteShowOptions{RemoteOptions: opts}
		sflags.BoolVar(&sopts.NoQuery, "n", false, "Do not query remotes with ls-remote")
		sflags.Parse(args[1:])
		args = sflags.Args()
		if len(args) < 1 {
			return printRemotes(c, opts)
		}
		return git.RemoteShow(c, sopts, git.Remote(args[0]), os.Stdout)
	default:
		return fmt.Errorf("Remote subcommand %v not implemented", args[0])
	}
}
