package cmd

import (
	"flag"
	"os"

	"github.com/driusan/dgit/git"
)

// Parses the command arguments from args (usually from os.Args) into a
// CheckoutIndexOptions and calls CheckoutIndex.
func CheckoutIndexCmd(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("checkout-index", flag.ExitOnError)
	options := git.CheckoutIndexOptions{}

	index := flags.Bool("index", false, "Update stat information for checkout out entries in the index")
	u := flags.Bool("u", false, "Alias for --index")

	quiet := flags.Bool("quiet", false, "Be quiet if files exist or are not in index")
	q := flags.Bool("q", false, "Alias for --quiet")

	force := flags.Bool("force", false, "Force overwrite of existing files")
	f := flags.Bool("f", false, "Alias for --force")

	all := flags.Bool("all", false, "Checkout all files in the index.")
	a := flags.Bool("a", false, "Alias for --all")

	nocreate := flags.Bool("no-create", false, "Don't checkout new files, only refresh existing ones")
	n := flags.Bool("n", false, "Alias for --no-create")

	flags.StringVar(&options.Prefix, "prefix", "", "When creating files, prepend string")
	flags.StringVar(&options.Stage, "stage", "", "Copy files from named stage (unimplemented)")

	flags.BoolVar(&options.Temp, "temp", false, "Instead of copying files to a working directory, write them to a temp dir")

	stdin := flags.Bool("stdin", false, "Instead of taking paths from command line, read from stdin")
	flags.BoolVar(&options.NullTerminate, "z", false, "Use nil instead of newline to terminate paths read from stdin")

	flags.Parse(args)
	files := flags.Args()
	options.UpdateStat = *index || *u
	options.Quiet = *quiet || *q
	options.Force = *force || *f
	options.All = *all || *a
	options.NoCreate = *nocreate || *n
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
