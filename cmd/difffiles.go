package cmd

import (
	"flag"
	"fmt"
	"log"

	"github.com/driusan/dgit/git"
)

func DiffFiles(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("diff-files", flag.ExitOnError)
	options := git.DiffFilesOptions{}

	patch := flags.Bool("patch", false, "Generate patch")
	p := flags.Bool("p", false, "Alias for --patch")
	u := flags.Bool("u", false, "Alias for --patch")

	nopatch := flags.Bool("no-patch", false, "Suppress patch generation")
	s := flags.Bool("s", false, "Alias of --no-patch")

	//unified := flags.Int("unified", 3, "Generate <n> lines of context")
	//U := flags.Int("U", 3, "Alias of --unified")
	flags.BoolVar(&options.Raw, "raw", true, "Generate the diff in raw format")

	flags.Parse(args)
	args = flags.Args()

	if *patch || *p || *u {
		options.Patch = true
		options.Raw = false
	}
	if *nopatch || *s {
		options.Patch = false
	}

	options.NumContextLines = 3

	diffs, err := git.DiffFiles(c, &options, args)
	if err != nil {
		return err
	}

	for _, diff := range diffs {
		if options.Raw {
			fmt.Printf("%v\n", diff)
		}
		if options.Patch {
			f, err := diff.Name.FilePath(c)
			if err != nil {
				log.Println(err)
			}
			patch, err := diff.ExternalDiff(c, diff.Src, diff.Dst, f, options.DiffCommonOptions)
			if err != nil {
				log.Print(err)
			} else {
				fmt.Printf("diff --git a/%v b/%v\n%v\n", diff.Name, diff.Name, patch)
			}
		}
	}
	return err
}
