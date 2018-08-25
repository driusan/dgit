package cmd

import (
	"flag"
	"fmt"

	"../git"
)

// WriteTree implements the git write-tree command on the Git repository
// pointed to by c.
func WriteTree(c *git.Client, args []string) (string, error) {
	flags := flag.NewFlagSet("write-tree", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\nOptions:\n\n")
		flags.PrintDefaults()
	}

	opts := git.WriteTreeOptions{}
	flags.BoolVar(&opts.MissingOk, "missing-ok", false, "allow missing objects")
	flags.StringVar(&opts.Prefix, "prefix", "", "write tree object for a subdirectory <prefix>")
	flags.Parse(args)

	sha1, err := git.WriteTree(c, opts)
	if err != nil {
		return "", err
	}
	return sha1.String(), nil
}
