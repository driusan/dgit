package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

func Archive(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("archive", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.ArchiveOptions{}

	// default compression level in the deflate package.
	opts.CompressionLevel = -1

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

	// FIXME: Find a better way to do this.
	var cl [10]bool
	flags.BoolVar(&cl[0], "0", false, "No compression")
	flags.BoolVar(&cl[1], "1", false, "Compress faster")
	flags.BoolVar(&cl[2], "2", false, "Compression level")
	flags.BoolVar(&cl[3], "3", false, "Compression level")
	flags.BoolVar(&cl[4], "4", false, "Compression level")
	flags.BoolVar(&cl[5], "5", false, "Compression level")
	flags.BoolVar(&cl[6], "6", false, "Compression level")
	flags.BoolVar(&cl[7], "7", false, "Compression level")
	flags.BoolVar(&cl[8], "8", false, "Compression level")
	flags.BoolVar(&cl[9], "9", false, "Highest compression level")

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

		// Parse the flags again skipping the first arg
		flags.Parse(args[1:])

		// After this second parse there should not be any arg left to parse.
		if flags.NArg() > 0 {
			return fmt.Errorf("<path> option not implemented")
		}
	} else if flags.NArg() == 1 {
		args := flags.Args()
		treeish = args[0]
	} else {
		flags.Usage()
		os.Exit(2)
	}

	// if a compression flag is set change the opts.CompressionLevel value.
	for i, v := range cl {
		if v {
			opts.CompressionLevel = i
			break
		}
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
		return fmt.Errorf("<path> option not implemented")
		// TODO: path option not implemented.
		//treeish = h[0]
		//opts.IncludePath = h[1]
	}

	return git.Archive(c, opts, treeish)
}
