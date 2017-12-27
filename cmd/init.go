package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

func Init(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("status", flag.ExitOnError)
	flags.Usage = func() {
		flag.Usage()
		flags.PrintDefaults()
	}

	opts := git.InitOptions{}

	flags.BoolVar(&opts.Bare, "bare", false, "Create bare repository")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Only print errors or warnings")
	q := flags.Bool("q", false, "Alias of --quiet")

	flags.Parse(args)
	if *q {
		opts.Quiet = true
	}
	args = flags.Args()
	var dir string
	switch len(args) {
	case 0:
		dir = "."
	case 1:
		dir = args[0]
	default:
		flags.Usage()
		return fmt.Errorf("Invalid init command. Must only provide one directory.")
	}

	_, err := git.Init(c, opts, dir)
	return err
}
