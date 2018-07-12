package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

func Status(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("status", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.StatusOptions{}

	flags.BoolVar(&opts.Short, "short", false, "Give the output in short format")
	s := flags.Bool("s", false, "Alias of --short")

	flags.BoolVar(&opts.Branch, "branch", false, "Short branch and tracking info, even in short mode")
	b := flags.Bool("b", false, "Alias of --branch")

	flags.BoolVar(&opts.ShowStash, "show-stash", false, "Show the number of entries currently stashed")

	porcelain := flags.Int("porcelain", 0, "Give the output in a porcelain format")

	flags.BoolVar(&opts.Long, "long", true, "Give the output in long format")

	flags.BoolVar(&opts.Verbose, "verbose", false, "In addition to the modifies files, show a diff")
	v := flags.Bool("v", false, "Alias of --verbose")

	uno := flags.Bool("uno", false, "Do not show untracked files")
	unormal := flags.Bool("unormal", false, "Show untracked files and directories (default)")
	uall := flags.Bool("uall", false, "Show untracked files and files inside of directories")

	// FIXME: This should handle both --ignore-submodules and --ignore-submodules=<when>
	ignoresubmodules := flags.String("ignore-submodules", "", "When to ignore submodules")

	flags.BoolVar(&opts.Ignored, "ignored", false, "Show ignored files as well")

	flags.BoolVar(&opts.NullTerminate, "z", false, "Terminate entries with NULL, not LF. Implies --porcelain=v1 if not specified")

	// FIXME: This should handle both --column and --column=<options>
	column := flags.String("column", "default", "Show status in columns (not implemented)")

	nocolumn := flags.Bool("no-column", false, "Equivalent to --column=never")

	flags.Parse(args)

	if *s {
		opts.Short = true
	}
	if *b {
		opts.Branch = true
	}

	switch *porcelain {
	case 0, 1, 2:
		opts.Porcelain = uint8(*porcelain)
	default:
		return fmt.Errorf("Invalid value for --porcelain")
	}

	if *v {
		opts.Verbose = true
	}

	if *uno {
		opts.UntrackedMode = git.StatusUntrackedNo
	} else if *unormal {
		opts.UntrackedMode = git.StatusUntrackedNormal
	} else if *uall {
		opts.UntrackedMode = git.StatusUntrackedAll
	} else {
		opts.UntrackedMode = git.StatusUntrackedNormal
	}

	switch *ignoresubmodules {
	case "", "all":
		opts.IgnoreSubmodules = git.StatusIgnoreSubmodulesAll
	case "none":
		opts.IgnoreSubmodules = git.StatusIgnoreSubmodulesNone
	case "untracked":
		opts.IgnoreSubmodules = git.StatusIgnoreSubmodulesUntracked
	case "dirty":
		opts.IgnoreSubmodules = git.StatusIgnoreSubmodulesAll
	default:
		return fmt.Errorf("Invalid option for --ignore-submodules")
	}

	if *column != "" {
		opts.Column = git.StatusColumnOptions(*column)
	}
	if *nocolumn {
		opts.Column = "never"
	}

	status, err := git.Status(c, opts, nil)
	if err != nil {
		return err
	}
	fmt.Print(status)
	return nil
}
