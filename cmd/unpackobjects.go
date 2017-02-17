package cmd

import (
	"flag"
	"os"

	"github.com/driusan/dgit/git"
)

// Parses the arguments from git-unpack-objects as they were passed on the commandline
// and calls git.CatFiles
func UnpackObjects(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("unpack-objects", flag.ExitOnError)
	options := git.UnpackObjectsOptions{}

	flags.BoolVar(&options.DryRun, "n", false, "Do not really unpack the objects")
	flags.BoolVar(&options.Quiet, "q", false, "Do not print progress information")
	flags.BoolVar(&options.Recover, "r", false, "Attempt to continue when dealing with a corrupt packfile")
	flags.BoolVar(&options.Strict, "strict", false, "Don't write objects with broken content or links")
	flags.UintVar(&options.MaxInputSize, "max-input-size", 0, "Do not process pack files larger than size")
	flags.Parse(args)

	_, err := git.UnpackObjects(c, options, os.Stdin)
	return err
}
