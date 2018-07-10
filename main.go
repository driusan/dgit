package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/driusan/dgit/cmd"
	"github.com/driusan/dgit/git"
	"os"
)

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
	dir := flag.String("C", "", "chdir before starting git")

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
		os.Exit(1)
	}
	if *dir != "" {
		if err := os.Chdir(*dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	c, err := git.NewClient(*gitdir, *workdir)
	subcommand = args[0]
	args = args[1:]

	if err != nil && requiresGitDir(subcommand) {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(3)
	}
	if c != nil && c.GitDir == "" && requiresGitDir(subcommand) {
		fmt.Fprint(os.Stderr, "Could not find .git directory\n")
		os.Exit(4)
	}

	switch subcommand {
	case "init":
		if err := cmd.Init(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "branch":
		cmd.Branch(c, args)
	case "checkout":
		if err := cmd.Checkout(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "checkout-index":
		if err := cmd.CheckoutIndexCmd(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "cat-file":
		if err := cmd.CatFile(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "add":
		if err := cmd.Add(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "commit":
		sha1, err := cmd.Commit(c, args)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		} else {
			fmt.Printf("%s\n", sha1)
		}
	case "commit-tree":
		sha1, err := cmd.CommitTree(c, args)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		} else {
			fmt.Printf("%s\n", sha1)
		}
	case "write-tree":
		sha1 := cmd.WriteTree(c, args)
		fmt.Printf("%s\n", sha1)
	case "update-ref":
		if err := cmd.UpdateRef(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "log":
		cmd.Log(c, args)
	case "symbolic-ref":
		val, err := cmd.SymbolicRef(c, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
		fmt.Printf("%s\n", val)
	case "clone":
		if err := cmd.Clone(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)

		}
	case "config":
		cmd.Config(c, args)
	case "fetch":
		cmd.Fetch(c, args)
	case "reset":
		if err := cmd.Reset(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
	case "merge-file":
		if err := cmd.MergeFile(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
	case "merge":
		if err := cmd.Merge(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
	case "merge-base":
		switch c, err := cmd.MergeBase(c, args); err {
		case cmd.Ancestor:
			os.Exit(0)
		case cmd.NonAncestor:
			os.Exit(1)
		default:
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(2)
			}
			fmt.Printf("%v\n", c)
		}
	case "rev-parse":
		commits, err := cmd.RevParse(c, args)
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
		cmd.RevList(c, args)
	case "hash-object":
		cmd.HashObject(c, args)
	case "status":
		if err := cmd.Status(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "ls-tree":
		if err := cmd.LsTree(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "push":
		cmd.Push(c, args)
	case "pack-objects":
		cmd.PackObjects(c, os.Stdin, args)
	case "send-pack":
		cmd.SendPack(c, args)
	case "read-tree":
		if err := cmd.ReadTree(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "diff":
		if err := cmd.Diff(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "diff-files":
		if err := cmd.DiffFiles(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "diff-index":
		if err := cmd.DiffIndex(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "diff-tree":
		if err := cmd.DiffTree(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "ls-files":
		if err := cmd.LsFiles(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "index-pack":
		if err := cmd.IndexPack(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "update-index":
		if err := cmd.UpdateIndex(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "unpack-objects":
		if err := cmd.UnpackObjects(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "grep":
		if err := cmd.Grep(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "apply":
		if err := cmd.Apply(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "revert":
		if err := cmd.Revert(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown git command %s.\n", subcommand)
	}
}
