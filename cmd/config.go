package cmd

import (
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Config(c *git.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: dgit config [<options>]\n")
		return
	}
	var fname string

	if args[0] == "--global" {
		home := os.Getenv("HOME")
		if home == "" {
			home = os.Getenv("home") // On some OSes, it is home
		}
		fname = home + "/.gitconfig"
		args = args[1:]
	} else {
		fname = c.GitDir.String() + "/config"
	}

	file, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		panic("Couldn't open config\n")
	}
	config := git.ParseConfig(file)
	err = file.Close()
	if err != nil {
		panic(err)
	}

	var action string
	switch args[0] {
	case "--get":
		action = "get"
		args = args[1:]
	case "--unset":
		action = "unset"
		args = args[1:]

	// Type canonicalization isn't currently supported
	//  and so we just strip them out and return the raw value
	//  with no validation
	case "--type":
		args = args[1:]
		fallthrough
	case "--bool":
		fallthrough
	case "--int":
		fallthrough
	case "--bool-or-int":
		fallthrough
	case "--path":
		fallthrough
	case "--expiry-date":
		if len(args) > 0 {
			args = args[1:]
		}
		fallthrough

	default:
		if len(args) == 1 {
			action = "get"
		} else if len(args) == 2 {
			action = "set"
		}
	}

	switch action {
	case "get":
		val, code := config.GetConfig(args[0])
		if code != 0 {
			os.Exit(code)
		}
		fmt.Printf("%s\n", val)
		os.Exit(0)
	case "set":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Missing value to set config to\n")
			return
		}

		config.SetConfig(args[0], args[1])
		err = os.Remove(fname)
		if err != nil {
			panic("Couldn't rewrite config\n")
		}
		outfile, err := os.Create(fname)
		if err != nil {
			panic("Couldn't open config\n")
		}
		defer outfile.Close()
		config.WriteFile(outfile)
		return
	case "unset":
		code := config.Unset(args[0])
		if code != 0 {
			os.Exit(code)
		}
		err = os.Remove(fname)
		if err != nil {
			panic("Couldn't rewrite config\n")
		}
		outfile, err := os.Create(fname)
		if err != nil {
			panic("Couldn't open config\n")
		}
		defer outfile.Close()
		config.WriteFile(outfile)
		os.Exit(0)
		return
	}

	panic("Unhandled action " + args[0])
}
