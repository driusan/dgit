package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

func LsTree(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("ls-tree", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.LsTreeOptions{}
	flags.BoolVar(&opts.TreeOnly, "d", true, "Show only the named tree, not its children")
	flags.BoolVar(&opts.Recurse, "r", false, "Recurse into sub-trees")
	flags.BoolVar(&opts.ShowTrees, "t", false, "Show trees even when recursing into them")
	flags.BoolVar(&opts.NullTerminate, "z", false, "\\0 line termination on output")
	flags.IntVar(&opts.Abbrev, "abbrev", 40, "Abbreviate hexidecimal identifiers to <abbrev> digits")

	long := flags.Bool("long", false, "Show size of blob entries")
	l := flags.Bool("l", false, "Alias of --long")
	flags.BoolVar(&opts.FullName, "full-name", false, "Show the full path name, not the pathname relative to the current working directory.")
	flags.BoolVar(&opts.FullTree, "full-tree", false, "Do not limit the listing to the current working directory.")
	nameonly := flags.Bool("name-only", false, "Only show the names of the files")
	namestatus := flags.Bool("name-status", false, "Only show the names of the files")

	flags.Parse(args)
	opts.NameOnly = *nameonly || *namestatus
	opts.Long = *long || *l

	args = flags.Args()
	if len(args) < 1 {
		flag.Usage()
		return fmt.Errorf("Missing tree")
	}

	treeID, err := git.RevParseTreeish(c, &git.RevParseOptions{}, args[0])
	if err != nil {
		return err
	}

	var files []git.File
	for _, file := range args[1:] {
		files = append(files, git.File(file))
	}

	tree, err := git.LsTree(c, opts, treeID, files)
	if err != nil {
		return err
	}
	for _, entry := range tree {
		var lineend string
		var name string
		if opts.FullName || opts.FullTree {
			name = entry.PathName.String()
		} else {
			rname, err := entry.PathName.FilePath(c)
			if err != nil {
				return err
			}
			name = rname.String()
		}
		if name == "." {
			// for compatibility with the real git client.
			name = "./"
		}
		if opts.NullTerminate {
			lineend = "\000"
		} else {
			lineend = "\n"
		}
		if !opts.NameOnly {
			if opts.Long {
				switch entry.Mode {
				case git.ModeBlob, git.ModeExec:
					fmt.Printf("%0.6o %s %s %7.d\t%s%s", entry.Mode, entry.Mode.TreeType(), entry.Sha1.String()[:opts.Abbrev], entry.Fsize, name, lineend)
				default:
					fmt.Printf("%0.6o %s %s %s\t%s%s", entry.Mode, entry.Mode.TreeType(), entry.Sha1.String()[:opts.Abbrev], "      -", name, lineend)
				}
			} else {
				fmt.Printf("%0.6o %s %s\t%s%s", entry.Mode, entry.Mode.TreeType(), entry.Sha1.String()[:opts.Abbrev], name, lineend)
			}
		} else {
			fmt.Printf("%s%s", name, lineend)
		}
	}
	return nil
}
