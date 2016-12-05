package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	libgit "github.com/driusan/git"
)

// Since libgit is somewhat out of our control and we can't implement
// a fmt.Stringer interface there, we use this helper.
func printCommit(c *libgit.Commit) {
	fmt.Printf("commit %v\nAuthor: %v\nDate: %v\n\n", c.Id, c.Author, c.Author.When.Format("Mon Jan 2 15:04:05 2006 -0700"))

	lines := strings.Split(c.CommitMessage, "\n")
	for _, l := range lines {
		fmt.Printf("    %v\n", l)
	}
}

func Log(c *Client, repo *libgit.Repository, args []string) error {
	if len(args) != 0 {
		fmt.Fprintf(os.Stderr, "Usage: go-git log\nNo options are currently supported.\n")
		return errors.New("No options are currently supported for log")
	}
	head, err := c.GetHeadID()
	if err != nil {
		return err
	}

	// CommitsBefore also returns the commit passed..
	l, err := repo.CommitsBefore(head)
	if err != nil {
		return err
	}
	for e := l.Front(); e != nil; e = e.Next() {
		c, ok := e.Value.(*libgit.Commit)
		if ok {
			printCommit(c)
		}
	}
	return nil

}
