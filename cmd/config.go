package cmd

import (
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Config(c *git.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: go-git config [<options>]\n")
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
	defer file.Close()

	config := git.ParseConfig(file)
	var action string
	switch args[0] {
	case "--get":
		action = "get"
		args = args[1:]
	case "--set":
		action = "set"
		args = args[1:]
	default:
		if len(args) == 1 {
			action = "get"
		} else if len(args) == 2 {
			action = "set"
		}
	}
	switch action {
	case "get":
		fmt.Printf("%s\n", config.GetConfig(args[0]))
		return
	case "set":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Missing value to set config to\n")
			return
		}

		file.Seek(0, 0)
		config.SetConfig(args[0], args[1])
		config.WriteFile(file)
		return
	}
	panic("Unhandled action" + args[0])
}
