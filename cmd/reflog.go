package cmd

import (
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Reflog(c *git.Client, args []string) error {
	// flags := newFlagSet("reflog")
	subcmd := "show"
	if len(args) > 0 {
		switch act := args[0]; act {
		case "show":
			// show should be an alias of log -g -abbrev-commit
			// -pretty=oneline, and just delegate there, but log
			// -g isn't implemented.
			subcmd = "show"
		case "expire":
			subcmd = "expire"
		case "delete":
			subcmd = "delete"
		case "exists":
			subcmd = "exists"
		default:
			if len(act) > 0 && act[0] == '-' {
				// It was an option, so fallback on show
				break
			}
			return fmt.Errorf("Invalid reflog subcommand: %v", args)
		}

	}
	switch subcmd {
	case "show", "delete":
		return fmt.Errorf("reflog subcommand %v not implemented", subcmd)
	case "expire":
		return fmt.Errorf("reflog subcommand %v not implemented", subcmd)
	case "exists":
		if len(args) != 2 {
			return fmt.Errorf("usage: %v reflog exists <ref>", os.Args[0])
		}
		if git.ReflogExists(c, git.Refname(args[1])) {
			os.Exit(0)
		}
		os.Exit(1)
		// without this go complains about no return at the end of
		// the function
		panic("unreachable")
	default:
		panic("Unknown reflog subcommand. This should be unreachable.")
	}

}
