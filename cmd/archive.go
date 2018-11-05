package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

func findArchiveFormat(input string) (git.ArchiveFormat, error) {
	// If the output is empty return the default value, a tarball.
	if input == "" {
		return git.ArchiveTar, nil
	}

	for k, v := range git.ArchiveFormatList() {
		if strings.HasSuffix(strings.ToLower(input), k) {
			return v, nil
		}
	}
	// The archive format is not found,
	// return tar by default and an error.
	return git.ArchiveTar, errors.New("Archive format not supported!")
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

	// Default archive format is tar.
	opts.Format = git.ArchiveTar

	// Default compression level in the deflate package.
	opts.CompressionLevel = -1

	flags.BoolVar(&opts.Verbose, "verbose", false, "Report archived files on stderr")
	flags.BoolVar(&opts.Verbose, "v", false, "Alias for --verbose")

	flags.StringVar(&opts.BasePrefix, "prefix", "", "Prepend prefix to each pathname in the archive")
	//flags.BoolVar(&opts.WorktreeAttributes, "worktree-attributes", false, "Not implemented")
	flags.Var(newNotimplBoolValue(), "worktree-attributes", "Not implemented")

	flags.BoolVar(&opts.List, "list", false, "List supported archive formats")
	flags.BoolVar(&opts.List, "l", false, "Alias for --list")

	flags.Var(newNotimplStringValue(), "remote", "Not implemented")
	flags.Var(newNotimplStringValue(), "exec", "Not implemented")

	var flagOutput, flagFormat string
	flags.StringVar(&flagOutput, "output", "", "Write the archive to this file")
	flags.StringVar(&flagOutput, "o", "", "Alias for --output")
	flags.StringVar(&flagFormat, "format", "", "Archive format")

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
	var paths []git.File

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

		// After this second parse the remaining args should be paths.
		for _, f := range flags.Args() {
			paths = append(paths, git.File(f))
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

	formatInput := flagOutput

	// If a --format is set use it instead
	if flagFormat != "" {
		formatInput = flagFormat
	}

	if formatInput != "" {
		format, err := findArchiveFormat(formatInput)

		// If we're trying to get the format from the --format flag
		// we must error if format is not supported.
		if err != nil && flagFormat != "" {
			return fmt.Errorf("Unknow archive format '%s'", flagFormat)
		}

		opts.Format = format

		// If the --output flag is not empty we must open/create the
		// output file.
		if flagOutput != "" {
			if file, err := os.Create(flagOutput); err != nil {
				return err
			} else {
				opts.OutputFile = file
				defer file.Close()
			}
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
		treeish = h[0]
		paths = append(paths, git.File(h[1]))
	}

	if tree, err := git.RevParseTreeish(c, &git.RevParseOptions{}, treeish); err != nil {
		return err
	} else {
		return git.Archive(c, opts, tree, paths)
	}
}
