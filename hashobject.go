package main

import (
	"bufio"
	"crypto/sha1"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

// Hashes the data of r with object type t, and returns
// the hash, and the data that was read from r.
func HashReader(t string, r io.Reader) (Sha1, []byte, error) {
	// Need to read the whole reader in order to find the size
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return Sha1{}, nil, err
	}

	h := sha1.New()
	fmt.Fprintf(h, "%s %d\000%s", t, len(data), data)
	s, err := Sha1FromSlice(h.Sum(nil))
	return s, data, err
}

func HashFile(t, filename string) (Sha1, []byte, error) {
	r, err := os.Open(filename)
	if err != nil {
		return Sha1{}, nil, err
	}
	defer r.Close()
	return HashReader(t, r)
}

func HashObject(c *Client, args []string) {
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
		fmt.Fprintln(os.Stderr, "Can not use both --stdin and --stdin-paths\n")
		return
	}

	if stdin {
		h, data, err := HashReader(t, os.Stdin)
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
			h, data, ferr := HashFile(t, val[:len(val)-1])
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
			h, data, err := HashFile(t, file)
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
