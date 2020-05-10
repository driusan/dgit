package cmd

import (
	"flag"
	"fmt"
	"strings"

	"github.com/driusan/dgit/git"
)

func DiffTree(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("diff-tree", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}
	options := git.DiffTreeOptions{}

	patch := flags.Bool("index", false, "Generate patch")
	p := flags.Bool("p", false, "Alias for --patch")
	u := flags.Bool("u", false, "Alias for --patch")

	nopatch := flags.Bool("no-patch", false, "Suppress patch generation")
	s := flags.Bool("s", false, "Alias of --no-patch")

	unified := flags.Int("unified", 3, "Generate <n> lines of context")
	U := flags.Int("U", 3, "Alias of --unified")
	flags.BoolVar(&options.Raw, "raw", true, "Generate the diff in raw format")
	flags.BoolVar(&options.Recurse, "r", false, "Recurse into subtrees")
	flags.BoolVar(&options.Root, "root", false, "Diff the initial commit against /dev/null")

	adjustedArgs := []string{}
	for _, a := range args {
		if strings.HasPrefix(a, "-U") && a != "-U" {
			adjustedArgs = append(adjustedArgs, "-U", a[2:])
			continue
		}
		adjustedArgs = append(adjustedArgs, a)
	}

	flags.Parse(adjustedArgs)
	args = flags.Args()

	if *patch || *p || *u {
		options.Patch = true
	}
	if *nopatch || *s {
		options.Patch = false
	}

	if unified != nil && U != nil && *unified != *U {
		return fmt.Errorf("Can not specify both --unified and -U %+v", adjustedArgs)
	} else if unified != nil {
		options.NumContextLines = *unified
	} else if U != nil {
		options.NumContextLines = *U
	}

	options.NumContextLines = 3

	if len(args) < 1 {
		flags.Usage()
		return fmt.Errorf("Must provide at least 1 treeish.")
	}
	treeish, err := git.RevParseTreeish(c, &git.RevParseOptions{}, args[0])
	if err != nil {
		return err
	}
	var treeish2 git.Treeish = nil
	var onetree bool = false
	if len(args) >= 2 {
		t2, err := git.RevParseTreeish(c, &git.RevParseOptions{}, args[1])
		if err != nil {
			return err
		}
		treeish2 = t2
		args = args[2:]
	} else {
		args = args[1:]
		onetree = true
	}
	diffs, err := git.DiffTree(c, &options, treeish, treeish2, args)
	if err != nil {
		return err
	}
	if onetree {
		if c1, ok := treeish.(git.Commitish); ok {
			cmt, err := c1.CommitID(c)
			if err != nil {
				// This shouldn't happen, the err
				// should have come from diff-tree
				// and been returned above, so panic
				// so that we investigate
				panic(err)
			}
			fmt.Println(cmt)
		}
	}

	for _, diff := range diffs {
		fmt.Printf("%v\n", diff)
	}
	return err
}
