package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

// Parses the arguments from git-cat-file as they were passed on the commandline
// and calls git.CatFiles
func CatFile(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("cat-file", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}
	options := git.CatFileOptions{}

	flags.BoolVar(&options.Pretty, "p", false, "Pretty print the object content")
	flags.BoolVar(&options.Size, "s", false, "Print the size of the object")
	flags.BoolVar(&options.Type, "t", false, "Print the type of the object")
	flags.BoolVar(&options.ExitCode, "e", false, "Exit with 0 status if file exists and is valid")
	flags.BoolVar(&options.AllowUnknownType, "allow-unknown-type", false, "Allow types that are unknown to git")
	flags.BoolVar(&options.Batch, "batch", false, "Read arguments in batch from stdin")
	flags.BoolVar(&options.BatchCheck, "batch-check", false, "Read arguments in batch from stdin")
	flags.StringVar(&options.BatchFmt, "batch-fmt", "", "Format string for batch or batch-check (non-standard)")

	adjustedArgs := make([]string, 0, len(args))

	for _, arg := range args {
		if strings.HasPrefix(arg, "-batch=") {
			adjustedArgs = append(adjustedArgs, "-batch")
			adjustedArgs = append(adjustedArgs, "-batch-fmt",
				strings.TrimPrefix(arg, "-batch="))
		} else if strings.HasPrefix(arg, "--batch=") {
			adjustedArgs = append(adjustedArgs, "-batch")
			adjustedArgs = append(adjustedArgs, "-batch-fmt",
				strings.TrimPrefix(arg, "--batch="))
		} else if strings.HasPrefix(arg, "-batch-check=") {
			adjustedArgs = append(adjustedArgs, "-batch-check")
			adjustedArgs = append(adjustedArgs, "-batch-fmt",
				strings.TrimPrefix(arg, "-batch-check="))
		} else if strings.HasPrefix(arg, "--batch-check=") {
			adjustedArgs = append(adjustedArgs, "-batch-check")
			adjustedArgs = append(adjustedArgs, "-batch-fmt",
				strings.TrimPrefix(arg, "--batch-check="))
		} else {
			adjustedArgs = append(adjustedArgs, arg)
		}
	}
	flags.Parse(adjustedArgs)

	oargs := flags.Args()

	switch len(oargs) {
	case 0:
		if options.Batch || options.BatchCheck {
			return git.CatFileBatch(c, options, io.TeeReader(os.Stdin, os.Stderr), os.Stdout)
		}

		flags.Usage()
		return nil
	case 1:
		if options.Batch || options.BatchCheck {
			return git.CatFileBatch(c, options, os.Stdin, os.Stdout)
		}

		shas, err := git.RevParse(c, git.RevParseOptions{}, oargs)
		if err != nil {
			return err
		}
		val, err := git.CatFile(c, "", shas[0].Id, options)
		if err != nil {
			return err
		}
		if options.Size || options.Type {
			fmt.Println(val)
		} else {
			fmt.Print(val)
		}
		return nil
	case 2:
		if options.Batch || options.BatchCheck {
			return fmt.Errorf("May not combine batch with type")
		}
		shas, err := git.RevParse(c, git.RevParseOptions{}, []string{oargs[1]})
		if err != nil {
			return err
		}
		val, err := git.CatFile(c, oargs[0], shas[0].Id, options)
		if err != nil {
			return err
		}
		fmt.Print(val)
		return nil
	default:
		flags.Usage()
	}
	return nil
}
