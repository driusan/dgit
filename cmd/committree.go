package cmd

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

func CommitTree(c *git.Client, args []string) (git.CommitID, error) {
	flags := flag.NewFlagSet("commit-tree", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	var p []string
	flags.Var(newMultiStringValue(&p), "p", "Each -p indicates the id of a parent commit object")
	var m []string
	flags.Var(newMultiStringValue(&m), "m", "A paragraph in the commit log messages. This can be given more than once.")
	messageFile := ""
	flags.StringVar(&messageFile, "F", "", "Read the commit log message from the given file.")

	// Commit-tree allows flags to go after the tree but flag package doesnt support it
	// We shift them to the beginning of the arguments list and parse again.
	extraFlags := []string{}
	newArgs := []string{}
	foundTree := false
	for idx, arg := range args {
		if foundTree && (arg == "-p" || arg == "-m" || arg == "-F" || len(extraFlags)%2 == 1) {
			extraFlags = append(extraFlags, arg)
		} else if foundTree {
			newArgs = append(newArgs, arg)
		} else if idx%2 == 0 && !strings.HasPrefix(arg, "-") {
			newArgs = append(newArgs, arg)
			foundTree = true
		}
	}

	args = args[:len(args)-len(newArgs)-len(extraFlags)]
	args = append(args, extraFlags...)
	args = append(args, newArgs...)

	flags.Parse(args)

	finalMessage := ""
	for _, msg := range m {
		finalMessage = msg + "\n"
	}

	if len(flags.Args()) != 1 {
		flags.Usage()
		os.Exit(2)
	}

	tree, err := git.RevParseTreeish(c, &git.RevParseOptions{}, flags.Arg(0))
	if err != nil {
		return git.CommitID{}, err
	}

	knownCommits := make(map[git.CommitID]bool)

	var parents []git.CommitID
	for _, parent := range p {
		pid, err := git.RevParseCommitish(c, &git.RevParseOptions{}, parent)
		if err != nil {
			return git.CommitID{}, err
		}

		pcid, err := pid.CommitID(c)
		if err != nil {
			return git.CommitID{}, err
		}

		if _, ok := knownCommits[pcid]; ok {
			// skip parents that have already been added
			continue
		}

		parents = append(parents, pcid)
		knownCommits[pcid] = true

	}

	if (finalMessage == "" && messageFile == "") || messageFile == "-" {
		// No -m or -F provided, read from STDIN
		m, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return git.CommitID{}, err
		}
		finalMessage = "\n" + string(m)
	} else if messageFile != "" {
		// No -m, but -F was provided. Read from file passed.
		m, err := ioutil.ReadFile(messageFile)
		if err != nil {
			return git.CommitID{}, err
		}
		finalMessage = "\n" + string(m)
	}

	return git.CommitTree(c, git.CommitTreeOptions{}, tree, parents, strings.TrimSpace(finalMessage))
}
