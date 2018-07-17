package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

func CommitTree(c *git.Client, args []string) (git.CommitID, error) {
	if len(args) == 0 || (len(args) == 1 && args[0] == "--help") {
		// This doesn't use the flag package, because -m or -p can be specified multiple times
		return git.CommitID{}, fmt.Errorf("Usage: %s commit-tree [(-p <sha1>)...] [-m <message>] [-F <file>]  <sha1>", os.Args[0])
	}

	var parents []git.CommitID
	var messageString, messageFile string
	var skipNext bool
	var tree git.Treeish
	knownCommits := make(map[git.CommitID]bool)
	for idx, val := range args {
		if idx == 0 && val[0] != '-' {
			treeid, err := git.RevParseTreeish(c, &git.RevParseOptions{}, args[len(args)-1])
			if err != nil {
				return git.CommitID{}, err
			}
			tree = treeid
			continue
		}

		if skipNext == true {
			skipNext = false
			continue
		}
		switch val {
		case "-p":
			pid, err := git.RevParseCommitish(c, &git.RevParseOptions{}, args[idx+1])
			if err != nil {
				return git.CommitID{}, err
			}
			pcid, err := pid.CommitID(c)
			if err != nil {
				return git.CommitID{}, err
			}
			skipNext = true

			if _, ok := knownCommits[pcid]; ok {
				// skip parents that have already been added
				continue
			}

			parents = append(parents, pcid)
			knownCommits[pcid] = true
		case "-m":
			messageString += "\n" + args[idx+1] + "\n"
			skipNext = true
		case "-F":
			messageFile = args[idx+1]
			skipNext = true
		}
	}
	if messageString == "" && messageFile == "" {
		// No -m or -F provided, read from STDIN
		m, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}
		messageString = "\n" + string(m)
	} else if messageString == "" && messageFile != "" {
		// No -m, but -F was provided. Read from file passed.
		m, err := ioutil.ReadFile(messageFile)
		if err != nil {
			panic(err)
		}
		messageString = "\n" + string(m)
	}

	if tree == nil {
		treeid, err := git.RevParseTreeish(c, &git.RevParseOptions{}, args[len(args)-1])
		if err != nil {
			return git.CommitID{}, err
		}
		tree = treeid
	}
	return git.CommitTree(c, git.CommitTreeOptions{}, tree, parents, strings.TrimSpace(messageString))
}
