package cmd

import (
	"errors"
	"fmt"
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
	//fmt.Printf("commit %v\nAuthor: %v\nDate: %v\n\n", c.Id, c.Author, c.Author.When.Format("Mon Jan 2 15:04:05 2006 -0700"))

	msg, err := cmt.GetCommitMessage(c)
	lines := strings.Split(strings.TrimSpace(msg), "\n")
	for _, l := range lines {
		fmt.Printf("    %v\n", l)
	}
	fmt.Printf("\n")
}

func Log(c *git.Client, args []string) error {
	if len(args) != 0 {
		fmt.Fprintf(os.Stderr, "Usage: go-git log\nNo options are currently supported.\n")
		return errors.New("No options are currently supported for log")
	}

	head, err := c.GetHeadCommit()
	if err != nil {
		return err
	}

	ancestors, err := head.Ancestors(c)
	if err != nil {
		return err
	}

	for _, cmt := range ancestors {
		printCommit(c, cmt)
	}
	return nil

}
