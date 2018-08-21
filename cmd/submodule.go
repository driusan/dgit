package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/driusan/dgit/git"
)

func Submodule(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("submodule", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	if len(args) < 1 || args[0] != "update" {
		flags.Usage()
		os.Exit(1)
	}

	//flags.Parse(args)

	// Scan for any .gitmodules files and fail with an error since none of this
	//  is supported yet.
	workdir := string(c.WorkDir)
	return filepath.Walk(workdir, func(path string, info os.FileInfo, err error) error {
		if filepath.Base(path) == ".gitmodules" {
			return fmt.Errorf("Submodules are not yet supported")
		}
		return nil
	})
}
