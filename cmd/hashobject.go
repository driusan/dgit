package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"../git"
)

func HashObject(c *git.Client, args []string) {
	flags := flag.NewFlagSet("hash-object", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	var t string
	var write, stdin, stdinpaths bool
	flags.StringVar(&t, "t", "blob", "-t object type")
	flags.BoolVar(&write, "w", false, "-w")
	flags.BoolVar(&stdin, "stdin", false, "--stdin to read an object from stdin")
	flags.BoolVar(&stdinpaths, "stdin-paths", false, "--stdin-paths to read a list of files from stdin")

	flags.Parse(args)

	if stdin && stdinpaths {
		fmt.Fprintln(flag.CommandLine.Output(), "Can not use both --stdin and --stdin-paths")
		flags.Usage()
		os.Exit(2)
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
		files := flags.Args()
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
