package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

var InvalidHead error = errors.New("Invalid HEAD")
var InvalidArgument error = errors.New("Invalid argument to function")

func requiresGitDir(cmd string) bool {
	switch cmd {
	case "init", "clone":
		return false
	default:
		return true
	}
}

var subcommand, subcommandUsage string

func main() {
	workdir := flag.String("work-tree", "", "specify the working directory of git")
	gitdir := flag.String("git-dir", "", "specify the repository of git")
	flag.Usage = func() {
		if subcommand == "" {
			subcommand = "subcommand"
		}
		if subcommandUsage == "" {
			subcommandUsage = fmt.Sprintf("%s [global options] %s [options]\n", os.Args[0], subcommand)
		}
		fmt.Fprintf(os.Stderr, "Usage: %s\n", subcommandUsage)
		fmt.Fprintf(os.Stderr, "\nGlobal options:\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(2)
	}
	c, err := NewClient(*gitdir, *workdir)
	subcommand = args[0]
	args = args[1:]

	if err != nil && requiresGitDir(subcommand) {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(3)
	}
	if c != nil && c.GitDir == "" && requiresGitDir(subcommand) {
		fmt.Fprintf(os.Stderr, "Could not find .git directory\n", err)
		os.Exit(4)
	}

	switch subcommand {
	case "init":
		Init(c, args)
	case "branch":
		Branch(c, args)
	case "checkout":
		Checkout(c, args)
	case "checkout-index":
		if err := CheckoutIndexCmd(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}

	case "add":
		Add(c, args)
	case "commit":
		sha1, err := Commit(c, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		} else {
			fmt.Printf("%s\n", sha1)
		}
	case "commit-tree":
		sha1, err := CommitTree(c, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		} else {
			fmt.Printf("%s\n", sha1)
		}
	case "write-tree":
		sha1 := WriteTree(c)
		fmt.Printf("%s\n", sha1)
	case "update-ref":
		UpdateRef(c, args)
	case "log":
		Log(c, args)
	case "symbolic-ref":
		val := SymbolicRef(c, args)
		fmt.Printf("%s\n", val)
	case "clone":
		Clone(c, args)
	case "config":
		Config(c, args)
	case "fetch":
		Fetch(c, args)
	case "reset":
		Reset(c, args)
	case "merge":
		if err := Merge(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
	case "merge-base":
		switch c, err := MergeBase(c, args); err {
		case Ancestor:
			os.Exit(0)
		case NonAncestor:
			os.Exit(1)
		default:
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(2)
			}
			fmt.Printf("%v\n", c)
		}
	case "rev-parse":
		commits, err := RevParse(c, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
		for _, sha := range commits {
			if sha.Excluded {
				fmt.Print("^")
			}
			fmt.Println(sha.Id.String())
		}

	case "rev-list":
		RevList(c, args)
	case "hash-object":
		HashObject(c, args)
	case "status":
		Status(c, args)
	case "ls-tree":
		err := LsTree(c, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "push":
		Push(c, args)
	case "pack-objects":
		PackObjects(c, os.Stdin, args)
	case "send-pack":
		SendPack(c, args)
	case "read-tree":
		ReadTree(c, args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown git command %s.\n", subcommand)
	}
}
