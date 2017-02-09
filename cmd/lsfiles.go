package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/go-git/git"
)

// Parses the arguments from git-ls-files as if they were passed on the commandline
// and calls git.LsFiles
func LsFiles(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("ls-tree", flag.ExitOnError)
	options := git.LsFilesOptions{}

	cached := flags.Bool("cached", true, "Show cached files in output (default)")
	ca := flags.Bool("c", true, "Alias for --cached")

	deleted := flags.Bool("deleted", false, "Show deleted files in output")
	d := flags.Bool("d", false, "Alias for --deleted")

	modified := flags.Bool("modified", false, "Show modified files in output")
	m := flags.Bool("m", false, "Alias of --modified")

	others := flags.Bool("other", false, "Show other (ie. untracked) files in output")
	o := flags.Bool("o", false, "Alias of --others")

	ignored := flags.Bool("ignored", false, "Show only ignored files in output")
	i := flags.Bool("i", false, "Alias of --ignored")

	stage := flags.Bool("stage", false, "Show staged content")
	s := flags.Bool("s", false, "Alias of --stage")

	flags.Parse(args)
	oargs := flags.Args()

	options.Cached = *cached || *ca

	rdeleted := *deleted || *d
	rmodified := *modified || *m
	rothers := *others || *o

	options.Deleted = rdeleted
	options.Modified = rmodified
	options.Others = rothers
	options.Ignored = *ignored || *i
	options.Stage = *stage || *s

	// If -u, -m or -o are given, cached is turned off.
	if rdeleted || rmodified || rothers {
		options.Cached = false
		// Check if --cache was explicitly given, in which case it shouldn't
		// have been turned off. (flag doesn't provide any way to differentiate
		// between "explicitly passed" and "default value")
		for _, arg := range args {
			if arg == "--cached" || arg == "-c" {
				options.Cached = true
				break
			}
		}
	}

	fmt.Printf("%v\n", options)
	files, err := git.LsFiles(c, &options, oargs)
	if err != nil {
		return err
	}
	for _, file := range files {
		if options.Stage {
			fmt.Printf("%o %v %v %v\n", file.Mode, file.Sha1, file.Stage(), file.PathName)
		} else {
			fmt.Printf("%v\n", file.PathName)
		}
	}
	return err
}
