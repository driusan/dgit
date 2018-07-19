package cmd

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/driusan/dgit/git"
)

func PackObjects(c *git.Client, input io.Reader, args []string) {
	flags := flag.NewFlagSet("pack-objects", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"q", "progress", "all-progress", "all-project-implied", "no-reuse-delta", "delta-base-offset", "non-empty", "local", "incremental", "revs", "unpacked", "all", "stdout", "shallow", "keep-true-parents"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"window", "depth", "keep-pack"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	flags.Parse(args)

	if flags.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	f, _ := os.Create(flags.Arg(0) + ".pack")
	defer f.Close()

	var objects []git.Sha1
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		b, err := hex.DecodeString(scanner.Text())
		if err != nil {
			panic(err)
		}
		s, err := git.Sha1FromSlice(b)
		if err != nil {
			panic(err)
		}
		objects = append(objects, s)
	}
	git.SendPackfile(c, f, objects)
}
