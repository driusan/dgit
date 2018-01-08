package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func HashObject(c *git.Client, args []string) {
	var t string
	var write, stdin, stdinpaths bool
	flag.StringVar(&t, "t", "blob", "-t object type")
	flag.BoolVar(&write, "w", false, "-w")
	flag.BoolVar(&stdin, "stdin", false, "--stdin to read an object from stdin")
	flag.BoolVar(&stdinpaths, "stdin-paths", false, "--stdin-paths to read a list of files from stdin")

	fakeargs := []string{"git-hash-object"}
	os.Args = append(fakeargs, args...)
	flag.Parse()

	if stdin && stdinpaths {
		fmt.Fprintln(os.Stderr, "Can not use both --stdin and --stdin-paths")
		return
	}

	if stdin {
		h, data, err := git.HashReader(t, os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}
		fmt.Printf("%s\n", h)
		if write {
			_, err := c.WriteObject(t, data)
			if err != nil {
				panic(err)
			}
		}
		return
	} else if stdinpaths {
		buffReader := bufio.NewReader(os.Stdin)
		for val, err := buffReader.ReadString('\n'); err == nil; val, err = buffReader.ReadString('\n') {
			// Trim the '\n' and hash the file.
			h, data, ferr := git.HashFile(t, val[:len(val)-1])
			if ferr != nil {
				fmt.Fprintf(os.Stderr, "%v\n", ferr)
				return
			}
			fmt.Printf("%s\n", h)
			if write {
				_, err := c.WriteObject(t, data)
				if err != nil {
					panic(err)
				}
			}

		}
		return
	} else {
		files := flag.Args()
		for _, file := range files {
			h, data, err := git.HashFile(t, file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
				return
			}

			fmt.Printf("%s\n", h)
			if write {
				_, err := c.WriteObject(t, data)
				if err != nil {
					panic(err)
				}
			}

		}
	}
}
