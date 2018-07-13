package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

// Parses the arguments from git-cat-file as they were passed on the commandline
// and calls git.CatFiles
func CatFile(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("cat-file", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	options := git.CatFileOptions{}

	flags.BoolVar(&options.Pretty, "p", false, "Pretty print the object content")
	flags.BoolVar(&options.Size, "s", false, "Print the size of the object")
	flags.BoolVar(&options.Type, "t", false, "Print the type of the object")
	flags.Parse(args)
	oargs := flags.Args()

	shas, err := git.RevParse(c, git.RevParseOptions{}, oargs)
	if err != nil {
		return err
	}
	for _, s := range shas {
		val, err := git.CatFile(c, s.Id, options)
		if err != nil {
			return err
		}
		fmt.Println(val)
	}
	return nil
}
