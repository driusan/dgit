package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/driusan/dgit/git"
)

func Init(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("status", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.InitOptions{}

	flags.BoolVar(&opts.Bare, "bare", false, "Create bare repository")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Only print errors or warnings")
	var template string
	flags.StringVar(&template, "template", "", "Specify the template directory that will be used")
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

	if template != "" {
		if !filepath.IsAbs(template) {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			template = filepath.Join(wd, template)
		}
		opts.Template = git.File(template)
	}

	_, err := git.Init(c, opts, dir)
	return err
}
