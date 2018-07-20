package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

// Parses the command arguments from args (usually from os.Args) into a
// CheckoutIndexOptions and calls CheckoutIndex.
func CheckoutIndexCmd(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("checkout-index", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}
	options := git.CheckoutIndexOptions{}

	flags.BoolVar(&options.UpdateStat, "index", false, "Update stat information for checkout out entries in the index")
	flags.BoolVar(&options.UpdateStat, "u", false, "Alias for --index")

	flags.BoolVar(&options.Quiet, "quiet", false, "Be quiet if files exist or are not in index")
	flags.BoolVar(&options.Quiet, "q", false, "Alias for --quiet")

	flags.BoolVar(&options.Force, "force", false, "Force overwrite of existing files")
	flags.BoolVar(&options.Force, "f", false, "Alias for --force")

	flags.BoolVar(&options.All, "all", false, "Checkout all files in the index.")
	flags.BoolVar(&options.All, "a", false, "Alias for --all")

	flags.BoolVar(&options.NoCreate, "no-create", false, "Don't checkout new files, only refresh existing ones")
	flags.BoolVar(&options.NoCreate, "n", false, "Alias for --no-create")

	flags.StringVar(&options.Prefix, "prefix", "", "When creating files, prepend string")
	flags.StringVar(&options.Stage, "stage", "", "Copy files from named stage (unimplemented)")

	flags.BoolVar(&options.Temp, "temp", false, "Instead of copying files to a working directory, write them to a temp dir")

	stdin := flags.Bool("stdin", false, "Instead of taking paths from command line, read from stdin")
	flags.BoolVar(&options.NullTerminate, "z", false, "Use nil instead of newline to terminate paths read from stdin")

	flags.Parse(args)
	files := flags.Args()
	if *stdin {
		options.Stdin = os.Stdin
	}

	// Convert from string to git.File
	gfiles := make([]git.File, len(files))
	for i, f := range files {
		gfiles[i] = git.File(f)
	}

	return git.CheckoutIndex(c, options, gfiles)

}
