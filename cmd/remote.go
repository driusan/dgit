package cmd

import (
	"fmt"
	"github.com/driusan/dgit/git"
)

func Remote(c *git.Client, args []string) error {
	flags := newFlagSet("remote")
	opts := git.RemoteOptions{}
	flags.BoolVar(&opts.Verbose, "v", false, "Make more verbose")
	flags.Parse(args)
	args = flags.Args()
	if len(args) < 1 {
		return fmt.Errorf("Missing remote subcommand")
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			return fmt.Errorf("Must provide name and URL for remote to add")
		}
		aopts := git.RemoteAddOptions{opts}
		return git.RemoteAdd(c, aopts, args[1], args[2])
	default:
		return fmt.Errorf("Remote subcommand %v not implemented", args[0])
	}
}
