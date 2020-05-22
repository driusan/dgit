package cmd

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/driusan/dgit/git"
)

func PackObjects(c *git.Client, input io.Reader, args []string) (rv error) {
	flags := flag.NewFlagSet("pack-objects", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	var opts git.PackObjectsOptions
	// These flags can be moved out of these lists and below as proper flags as they are implemented
	for _, bf := range []string{"q", "progress", "all-progress", "all-project-implied", "no-reuse-delta", "non-empty", "local", "incremental", "revs", "unpacked", "all", "stdout", "shallow", "keep-true-parents"} {
		flags.Var(newNotimplBoolValue(), bf, "Not implemented")
	}
	for _, sf := range []string{"depth", "keep-pack"} {
		flags.Var(newNotimplStringValue(), sf, "Not implemented")
	}

	flags.IntVar(&opts.Window, "window", 10, "Size of the sliding window to use for delta calculation")
	flags.BoolVar(&opts.DeltaBaseOffset, "delta-base-offset", false, "Use offset deltas instead of ref deltas in pack")

	flags.Parse(args)

	if flags.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	var trailer git.Sha1
	dir := filepath.Dir(flags.Arg(0))
	f, err := ioutil.TempFile(dir, "dgitpack")
	if err != nil {
		return err
	}
	defer func() {
		if rv != nil {
			f.Close()
			os.Remove(f.Name())
		} else {
			if err := os.Rename(f.Name(), fmt.Sprintf("%s-%s.pack", flags.Arg(0), trailer)); err != nil {
				rv = err
				return
			}
			idx, err := os.Create(fmt.Sprintf("%s-%s.idx", flags.Arg(0), trailer))
			if err != nil {
				rv = err
				return
			}
			var iopts git.IndexPackOptions
			iopts.Output = idx
			f.Seek(0, io.SeekStart)
			if _, err := git.IndexPack(c, iopts, f); err != nil {
				rv = err
			}
			f.Close()
			idx.Close()
			return
		}
	}()

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
	trailer, rv = git.PackObjects(c, opts, f, objects)
	fmt.Printf("%s", trailer)
	return
}
