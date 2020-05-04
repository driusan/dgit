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

		aopts := git.RemoteAddOptions{RemoteOptions: opts}
		addflags := newFlagSet("remote add")
		addflags.BoolVar(&aopts.Fetch, "fetch", false, "fetch the remote branches")
		addflags.BoolVar(&aopts.Fetch, "f", false, "Alias of --fetch")

		addflags.BoolVar(&aopts.Fetch, "tags", false, "Import all tags and associated objects when fetching")

		addflags.StringVar(&aopts.Track, "track", "", "branches to track")
		addflags.StringVar(&aopts.Track, "t", "", "Alias of --track")

		addflags.StringVar(&aopts.Master, "master", "", "master branch")
		addflags.StringVar(&aopts.Master, "m", "", "Alias of --master")

		addflags.StringVar(&aopts.Master, "mirror", "", "Set up remote as a mirror to push to or fetch from")

		addflags.Parse(args[1:])
		args = addflags.Args()

		fmt.Printf("%v", args)
		if len(args) < 2 {
			return fmt.Errorf("Must provide name and URL for remote to add")
		}

		// Go sometimes runs "git remote add name -- url" with the
		// misplaced -- for no reason other than because it can.
		// As a hack, remote any "--" in the leftover arguments
		// so that go get will work.
		newargs := make([]string, 0, len(args))
		for _, arg := range args {
			if arg == "--" {
				continue
			}
			newargs = append(newargs, arg)
		}
		return git.RemoteAdd(c, aopts, newargs[0], newargs[1])
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
