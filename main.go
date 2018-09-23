package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/driusan/dgit/cmd"
	"github.com/driusan/dgit/git"
	"io/ioutil"
	"log"
	"os"
	"strings"
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
	// First thing, set up logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	traceLevel := os.Getenv("DGIT_TRACE")
	if traceLevel == "" {
		traceLevel = os.Getenv("GIT_TRACE")
	}

	// If no trace level specified then just dump any log output
	if traceLevel == "" {
		log.SetOutput(ioutil.Discard)
	} else if traceLevel != "1" && traceLevel != "2" {
		logfile, err := os.OpenFile(traceLevel, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

		if err != nil {
			fmt.Printf("Could not open file %v for tracing: %v\n", traceLevel, err)
			os.Exit(1)
		}

		log.SetOutput(logfile)
	}

	wd, _ := os.Getwd()

	log.Printf("Dgit started: (%v) %v\n", wd, os.Args)

	// Special case, just reverse the arguments to force regular --help handling
	if len(os.Args) == 3 && os.Args[1] == "help" && !strings.HasPrefix(os.Args[2], "-") {
		os.Args[1] = os.Args[2]
		os.Args[2] = "--help"
		flag.CommandLine.SetOutput(os.Stdout)
	} else if len(os.Args) == 3 && os.Args[2] == "--help" {
		flag.CommandLine.SetOutput(os.Stdout)
	}

	workdir := flag.String("work-tree", "", "specify the working directory of git")
	gitdir := flag.String("git-dir", "", "specify the repository of git")
	dir := flag.String("C", "", "chdir before starting git")
	superprefix := flag.String("super-prefix", "", "useless option used internally by git test suite")

	flag.Usage = func() {
		if subcommand == "" {
			subcommand = "subcommand"
		}
		subcommandUsage = fmt.Sprintf("%s [global options] %s [options] %s", os.Args[0], subcommand, subcommandUsage)
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s\n\n", subcommandUsage)
		fmt.Fprintf(flag.CommandLine.Output(), "\nGlobal options:\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		flag.CommandLine.SetOutput(os.Stdout)
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
	if c != nil {
		defer c.Close()
	}
	if *superprefix != "" {
		c.SuperPrefix = *superprefix
	}

	switch subcommand {
	case "init":
		if err := cmd.Init(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "branch":
		subcommandUsage = "[branchname]"
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
		sha1, err := cmd.WriteTree(c, args)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		} else {
			fmt.Printf("%s\n", sha1)
		}
	case "mktree":
		if err := cmd.MkTree(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "update-ref":
		if err := cmd.UpdateRef(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "log":
		subcommandUsage = "[commitish]"
		err := cmd.Log(c, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "symbolic-ref":
		val, err := cmd.SymbolicRef(c, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
		fmt.Printf("%s\n", val)
	case "clone":
		subcommandUsage = "<repository> [<directory>]"
		if err := cmd.Clone(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)

		}
	case "config":
		subcommandUsage = "name [value]"
		if err := cmd.Config(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
	case "fetch":
		subcommandUsage = "[<repository>]"
		if err := cmd.Fetch(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
	case "pull":
		subcommandUsage = "[<repository [<refspec>...]]"
		if err := cmd.Pull(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
	case "reset":
		if err := cmd.Reset(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
	case "merge-file":
		subcommandUsage = "<current-file> <base-file> <other-file>"
		if err := cmd.MergeFile(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
	case "merge":
		subcommandUsage = "<commit>..."
		if err := cmd.Merge(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
	case "merge-base":
		subcommandUsage = "<commit>..."
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
		subcommandUsage = "<args>..."
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
		subcommandUsage = "<commit>..."
		if err := cmd.RevList(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "rm":
		subcommandUsage = "-- <file>..."
		if err := cmd.Rm(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "hash-object":
		subcommandUsage = "[<file>...]"
		cmd.HashObject(c, args)
	case "status":
		if err := cmd.Status(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "ls-tree":
		subcommandUsage = "[<path>...]"
		if err := cmd.LsTree(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "push":
		subcommandUsage = "<repository>"
		if err := cmd.Push(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "pack-objects":
		subcommandUsage = "<basename>"
		cmd.PackObjects(c, os.Stdin, args)
	case "send-pack":
		cmd.SendPack(c, args)
	case "read-tree":
		if err := cmd.ReadTree(c, args); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(4)
		}
	case "diff":
		subcommandUsage = "[<path>...]"
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
		subcommandUsage = "[<file>...]"
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
		subcommandUsage = "[<pathspec>...]"
		if err := cmd.Grep(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "apply":
		subcommandUsage = "[<patch>...]"
		if err := cmd.Apply(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "revert":
		subcommandUsage = "<commit>..."
		if err := cmd.Revert(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "show":
		subcommandUsage = "<commit>..."
		if err := cmd.Show(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "mktag":
		tagid, err := cmd.Mktag(c, args)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(tagid)
	case "tag":
		if err := cmd.Tag(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "var":
		subcommandUsage = "[variable]"
		if err := cmd.Var(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "check-ignore":
		subcommandUsage = "[<pathname>...]"
		if err := cmd.CheckIgnore(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(128)
		}
	case "submodule":
		subcommandUsage = "update"
		if err := cmd.Submodule(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "show-ref":
		subcommandUsage = "[<pattern>...]"
		if err := cmd.ShowRef(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case "ls-remote":
		subcommandUsage = "[repo [<patterns>..]]"
		if err := cmd.LsRemote(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "clean":
		if err := cmd.Clean(c, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "help":
		flag.CommandLine.SetOutput(os.Stdout)
		flag.Usage()

		fmt.Fprintf(flag.CommandLine.Output(), "\nAvailable subcommands:\n")
		fmt.Fprintf(flag.CommandLine.Output(), `
   init
   branch
   checkout
   checkout-index
   cat-file
   add          
   commit         
   commit-tree
   write-tree
   update-ref
   log              
   symbolic-ref
   clone          Clone a repository into a new directory   
   config
   fetch          Download objects and refs from another repository
   reset
   merge-file
   merge
   merge-base
   rev-parse
   rev-list
   hash-object
   status
   ls-tree
   pull           Fetch from and integrate with another repository or a local branch
   push
   pack-objects
   send-pack
   read-tree
   diff
   diff-files
   diff-index
   diff-tree
   ls-files
   index-pack
   update-index
   unpack-objects
   grep
   apply
   revert
   help
   show             Show various types of objects
   var              Show a Git logical variable
   submodule        Initialize, update or inspect submodules
   showref          List references in a local repository
`)

		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown git command %s.\n", subcommand)
		os.Exit(1)
	}
}
