package cmd

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/driusan/dgit/git"
)

func argsParseCompressionLevel(opts *git.ArchiveOptions, args []string) []string {
	for i, arg := range args {
		if len(arg) < 2 || arg[0] != '-' {
			continue
		}
		startIndex := 1
		if arg[1] == '-' {
			startIndex++
			if len(arg) == 2 {
				continue
			}
		}

		// slice the dash
		value := arg[startIndex:]

		// Try to convert to integer
		if v, err := strconv.Atoi(value); err != nil {
			// The value is NaN
			continue
		} else {
			// The compression value must be between 0..9
			if v >= 0 && v <= 9 {
				opts.CompressionLevel = v
			}
			// remove this value from the arg list
			return append(args[:i], args[i+1:]...)
		}
	}
	return args
}

func Archive(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("archive", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.ArchiveOptions{}

	//flags.BoolVar(&opts.Verbose, "verbose", false, "Report archived files on stderr")
	//flags.BoolVar(&opts.Verbose, "v", false, "Alias for --verbose")
	flags.Var(newNotimplBoolValue(), "verbose", "Not implemented")
	flags.Var(newNotimplBoolValue(), "v", "Not implemented")

	flags.StringVar(&opts.BasePrefix, "prefix", "", "Prepend prefix to each pathname in the archive")
	flags.StringVar(&opts.OutputFile, "output", "", "Write the archive to this file")
	flags.StringVar(&opts.OutputFile, "o", "", "Alias for --output")

	//flags.BoolVar(&opts.WorktreeAttributes, "worktree-attributes", false, "Not implemented")
	flags.Var(newNotimplBoolValue(), "worktree-attributes", "Not implemented")

	flags.BoolVar(&opts.List, "list", false, "List supported archive formats")
	flags.BoolVar(&opts.List, "l", false, "Alias for --list")

	flags.Var(newNotimplStringValue(), "remote", "Not implemented")
	flags.Var(newNotimplStringValue(), "exec", "Not implemented")

	flags.StringVar(&opts.ArchiveFormat, "format", "", "Archive format")

	flags.IntVar(&opts.CompressionLevel, "0", 0, "Store only")
	flags.IntVar(&opts.CompressionLevel, "9", 9, "Highest compression level")

	// if the args is empty, print usage.
	if len(args) == 0 {
		flags.Usage()
		os.Exit(2)
	}

	flags.Parse(args)

	var treeish string

	// Since the Flag parsing stops before the first non-flag argument
	// there can be remaining arguments to parse.
	// For example calling dgit with the followings args
	// "dgit archive HEAD -o test.tar" the -o flag will not be parsed
	// so we must parse the flags again if there are remaining args to parse.
	if flags.NArg() > 1 {
		args := flags.Args()

		// The first not parsed arg should be the treeish
		treeish = args[0]

		// Check if there's a -1..8 compression level flag
		args = argsParseCompressionLevel(&opts, args[1:])

		// Parse the flags again skipping the first arg
		flags.Parse(args)

		// After this second parse there should not be any arg left to parse.
		if flags.NArg() > 0 {
			flags.Usage()
			os.Exit(2)
		}
	} else if flags.NArg() == 1 {
		args := flags.Args()
		treeish = args[0]
	}

	if opts.List {
		formatList := git.ArchiveFormatList()
		for _, f := range formatList {
			fmt.Println(f)
		}
		return nil
	}

	// Special case for "HEAD:folder/"
	if h := strings.SplitN(treeish, ":", 2); len(h) == 2 {
		return fmt.Errorf("<path> option not implemented.")
		// TODO: path option not implemented.
		//treeish = h[0]
		//opts.IncludePath = h[1]
	}

	return git.Archive(c, opts, treeish)
}
