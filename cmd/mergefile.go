package cmd

import (
	"flag"
	"fmt"
	"github.com/driusan/dgit/git"
	"io"
	"os"
)

func MergeFile(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("merge-file", flag.ExitOnError)
	flags.SetOutput(os.Stdout)
	options := git.MergeFileOptions{}

	flags.Parse(args)
	args = flags.Args()

	if len(args) <= 1 {
		flag.Usage()
		return fmt.Errorf("Invalid usage of merge-file")
	}

	// -L may be specified multiple times, so we need to manually look for it instead of
	// using flag
	nEls := 0
LabelLoop:
	for args[0] == "-L" {
		if len(args) < 2 {
			break LabelLoop
		}
		switch nEls {
		case 0:
			options.Current.Label = args[1]
		case 1:
			options.Base.Label = args[1]

		case 2:
			options.Other.Label = args[1]
		default:
			flag.Usage()
			return fmt.Errorf("May only specify -L up to three times.")
		}
		nEls++
	}
	if len(args) != 3 {
		flag.Usage()
		return fmt.Errorf("Invalid usage of merge-file")
	}
	for i, file := range args {
		switch i {
		case 0:
			options.Current.Filename = git.File(file)
		case 1:
			options.Base.Filename = git.File(file)
		case 2:
			options.Other.Filename = git.File(file)
		}
	}
	newcontent, err := git.MergeFile(c, options)
	if newcontent != nil {
		if options.Stdout {
			io.Copy(os.Stdout, newcontent)
		} else {
			f, err := os.Create(options.Current.Filename.String())
			if err != nil {
				println("Error opening")
				return err
			}

			io.Copy(f, newcontent)

			f.Close()
		}
	}
	return err
}
