package cmd

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/driusan/dgit/git"
)

// WriteTree implements the git write-tree command on the Git repository
// pointed to by c.
func WriteTree(c *git.Client, args []string) string {
	flags := flag.NewFlagSet("write-tree", flag.ExitOnError)
        flags.SetOutput(os.Stdout)
	flags.Usage = func() {
		//fmt.Fprintf(os.Stderr, "usage: %v write-tree [--missing-ok] [--prefix <prefix>/]\n\n", os.Args[0])
		flag.Usage()
		fmt.Fprintf(os.Stderr, "\nOptions:\n\n")
		flags.PrintDefaults()
	}

	opts := git.WriteTreeOptions{}
	flags.BoolVar(&opts.MissingOk, "missing-ok", false, "allow missing objects")
	flags.StringVar(&opts.Prefix, "prefix", "", "write tree object for a subdirectory <prefix>")
	flags.Parse(args)

	sha1, err := git.WriteTree(c, opts)
	if err != nil {
		log.Fatal(err)
	}
	return sha1.String()
}
