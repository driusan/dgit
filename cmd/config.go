package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Config(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("config", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	get := flags.Bool("get", false, "Get the value for a given key")
	unset := flags.Bool("unset", false, "Remove the line matching the key")
	list := flags.Bool("list", false, "List all variables along with their values")
	global := flags.Bool("global", false, "For writing options: write to global file rather than respository")

	// Type canonicalization isn't currently supported
	//  and so we just allow them and return the raw value
	//  with no validation
	flags.String("type", "", "")
	flags.Bool("bool", false, "")
	flags.Bool("int", false, "")
	flags.Bool("bool-or-int", false, "")
	flags.Bool("path", false, "")
	flags.Bool("expiry-date", false, "")

	flags.Parse(args)

	var fname string

	if *global {
		home := os.Getenv("HOME")
		if home == "" {
			home = os.Getenv("home") // On some OSes, it is home
		}
		fname = home + "/.gitconfig"
	} else {
		fname = c.GitDir.String() + "/config"
	}

	file, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	config := git.ParseConfig(file)
	err = file.Close()
	if err != nil {
		return err
	}

	var action string
	if *get {
		action = "get"
	} else if *unset {
		action = "unset"
	} else if *list {
		action = "list"
	} else if flags.NArg() == 1 {
		action = "get"
	} else if flags.NArg() == 2 {
		action = "set"
	}

	switch action {
	case "get":
		if flags.NArg() < 1 {
			fmt.Fprintf(flag.CommandLine.Output(), "Missing value to get\n")
			flags.Usage()
			os.Exit(2)
		}
		val, code := config.GetConfig(flags.Arg(0))
		if code != 0 {
			os.Exit(code)
		}
		fmt.Printf("%s\n", val)
		return nil
	case "set":
		if flags.NArg() < 2 {
			fmt.Fprintf(flag.CommandLine.Output(), "Missing value to set config to\n")
			flags.Usage()
			os.Exit(2)
		}

		config.SetConfig(flags.Arg(0), flags.Arg(1))
		err = os.Remove(fname)
		if err != nil {
			return err
		}
		outfile, err := os.Create(fname)
		if err != nil {
			return err
		}
		defer outfile.Close()
		config.WriteFile(outfile)
		return nil
	case "unset":
		if flags.NArg() < 1 {
			fmt.Fprintf(flag.CommandLine.Output(), "Missing value to unset\n")
			flags.Usage()
			os.Exit(2)
		}
		code := config.Unset(flags.Arg(0))
		if code != 0 {
			os.Exit(code)
		}
		err = os.Remove(fname)
		if err != nil {
			return err
		}
		outfile, err := os.Create(fname)
		if err != nil {
			return err
		}
		defer outfile.Close()
		config.WriteFile(outfile)
		return nil
	case "list":
		list := config.GetConfigList()
		for _, entry := range list {
			fmt.Printf("%s\n", entry)
		}
		return nil
	}

	flags.Usage()
	os.Exit(2)

	return errors.New("Unhandled action")
}
