package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"../git"
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
	flags.BoolVar(&opts.Quiet, "q", false, "Alias of --quiet")
	var template string
	flags.StringVar(&template, "template", "", "Specify the template directory that will be used")

	flags.Parse(args)
	args = flags.Args()
	var dir string
	switch len(args) {
	case 0:
		dir = "."
	case 1:
		dir = args[0]
	default:
		fmt.Fprintf(flag.CommandLine.Output(), "Invalid init command. Must only provide one directory.\n")
		flags.Usage()
		os.Exit(2)
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
