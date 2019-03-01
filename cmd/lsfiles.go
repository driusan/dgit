package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

// Parses the arguments from git-ls-files as if they were passed on the commandline
// and calls git.LsFiles
func LsFiles(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("ls-files", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}
	options := git.LsFilesOptions{}

	cached := flags.Bool("cached", true, "Show cached files in output (default)")
	ca := flags.Bool("c", true, "Alias for --cached")

	deleted := flags.Bool("deleted", false, "Show deleted files in output")
	d := flags.Bool("d", false, "Alias for --deleted")

	modified := flags.Bool("modified", false, "Show modified files in output")
	m := flags.Bool("m", false, "Alias of --modified")

	others := flags.Bool("others", false, "Show other (ie. untracked) files in output")
	o := flags.Bool("o", false, "Alias of --others")

	ignored := flags.Bool("ignored", false, "Show only ignored files in output")
	i := flags.Bool("i", false, "Alias of --ignored")

	stage := flags.Bool("stage", false, "Show staged content")
	s := flags.Bool("s", false, "Alias of --stage")

	unmerged := flags.Bool("unmerged", false, "Show unmerged files. Implies --stage")
	u := flags.Bool("u", false, "Alias of --unmerged")

	flags.BoolVar(&options.Killed, "killed", false, "Show files that need to be removed for checkout-index to succeed")
	flags.BoolVar(&options.Killed, "k", false, "Alias of --killed")

	flags.BoolVar(&options.Status, "t", false, "Show status of files")

	flags.BoolVar(&options.ExcludeStandard, "exclude-standard", false, "Add the standard Git exclusions.")

	flags.BoolVar(&options.Directory, "directory", false, "Show only directory, not its contents if a directory is untracked")
	flags.BoolVar(&options.NoEmptyDirectory, "no-empty-directory", false, "Do not show empty untracked directories in output")

	var excludefiles, excludeperdirectory []string
	flags.Var(NewMultiStringValue(&options.ExcludePatterns), "exclude", "Skip untracked files matching pattern.")
	flags.Var(NewMultiStringValue(&options.ExcludePatterns), "x", "Alias for --exclude")

	flags.Var(NewMultiStringValue(&excludefiles), "exclude-from", "Read exclude patterns from a file.")
	flags.Var(NewMultiStringValue(&excludefiles), "X", "Alias for --exclude-from")

	flags.Var(NewMultiStringValue(&excludeperdirectory), "exclude-per-directory", "Read additional exclude patterns for each directory.")

	flags.BoolVar(&options.ErrorUnmatch, "error-unmatch", false, "Exit with an error if any unmatched paths are specified on the command line")

	flags.Parse(args)
	oargs := flags.Args()

	for _, f := range excludefiles {
		options.ExcludeFiles = append(options.ExcludeFiles, git.File(f))
	}
	for _, f := range excludeperdirectory {
		options.ExcludePerDirectory = append(options.ExcludePerDirectory, git.File(f))
	}
	options.Cached = *cached || *ca

	rdeleted := *deleted || *d
	rmodified := *modified || *m
	rothers := *others || *o
	runmerged := *unmerged || *u

	options.Deleted = rdeleted
	options.Modified = rmodified
	options.Others = rothers
	options.Ignored = *ignored || *i
	options.Stage = *stage || *s
	if runmerged {
		options.Unmerged = true
		options.Stage = true
	}

	// If -u, -m or -o are given, cached is turned off.
	if rdeleted || rmodified || rothers || runmerged {
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

	rargs := make([]git.File, len(oargs), len(oargs))
	for i := range oargs {
		rargs[i] = git.File(oargs[i])
	}

	files, err := git.LsFiles(c, options, rargs)
	if err != nil {
		return err
	}

	// LsFiles converted them to IndexEntries so that it could return the
	// stage, sha1, and mode if --stage was passed. We need to convert them
	// back to relative paths.
	for _, file := range files {
		path, err := file.PathName.FilePath(c)
		if err != nil {
			return err
		}
		if options.Status {
			fmt.Printf("%c ", file.StatusCode)
		}
		if options.Stage {
			fmt.Printf("%o %v %v\t%v\n", file.Mode, file.Sha1, file.Stage(), path)
		} else {
			if options.Directory && path.IsDir() {
				fmt.Println(path + "/")
			} else {
				fmt.Println(path)
			}
		}
	}
	return err
}
