package cmd

import (
	"errors"
	"fmt"
        "log"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

// Since libgit is somewhat out of our control and we can't implement
// a fmt.Stringer interface there, we use this helper.
func printCommit(c *git.Client, cmt git.CommitID) {
	author, err := cmt.GetAuthor(c)
	if err != nil {
		panic(err)
	}
	fmt.Printf("commit %s\n", cmt)
	if parents, err := cmt.Parents(c); len(parents) > 1 && err == nil {
		fmt.Printf("Merge: ")
		for i, p := range parents {
			fmt.Printf("%s", p)
			if i != len(parents)-1 {
				fmt.Printf(" ")
			}
		}
		fmt.Println()
	}
	date, err := cmt.GetDate(c)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Author: %v\nDate:   %v\n\n", author, date.Format("Mon Jan 2 15:04:05 2006 -0700"))
	log.Printf("Commit %v\n", cmt)

	msg, err := cmt.GetCommitMessage(c)
	lines := strings.Split(strings.TrimSpace(msg.String()), "\n")
	for _, l := range lines {
		fmt.Printf("    %v\n", l)
	}
	fmt.Printf("\n")
}

var visited map[git.CommitID]bool

func walkParents(c *git.Client, cmt git.CommitID) error {
	if visited[cmt] {
		return nil
	}
	visited[cmt] = true
	printCommit(c, cmt)
	parents, err := cmt.Parents(c)
	if err != nil {
		return err
	}
	for _, p := range parents {
		if _, visited := visited[p]; visited {
			continue
		}
		if err := walkParents(c, p); err != nil {
			return err
		}
	}
	return nil
}

func Log(c *git.Client, args []string) error {
	if len(args) >= 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s log [commitish]\n", os.Args[0])
		return errors.New("No options are currently supported for log")
	}

	var commit git.Commitish
	var err error
	if len(args) == 0 {
		commit, err = git.RevParseCommitish(c, &git.RevParseOptions{}, "HEAD")
	} else {
		commit, err = git.RevParseCommitish(c, &git.RevParseOptions{}, args[0])
	}
	if err != nil {
		return err
	}

	cmt, err := commit.CommitID(c)
	if err != nil {
		return err
	}

	visited = make(map[git.CommitID]bool)

	return walkParents(c, cmt)

}
