package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/driusan/dgit/git"
	//	libgit "github.com/driusan/git"
)

var dateCache map[git.CommitID]time.Time

// Since libgit is somewhat out of our control and we can't implement
// a fmt.Stringer interface there, we use this helper.
func printCommit(c *git.Client, cmt git.CommitID) {
	author, err := cmt.GetAuthor(c)
	if err != nil {
		panic(err)
	}
	fmt.Printf("commit %s\nAuthor: %v\nDate: %v\n\n", cmt, author, dateCache[cmt].Format("Mon Jan 2 15:04:05 2006 -0700"))
	//fmt.Printf("commit %v\nAuthor: %v\nDate: %v\n\n", c.Id, c.Author, c.Author.When.Format("Mon Jan 2 15:04:05 2006 -0700"))

	msg, err := cmt.GetCommitMessage(c)
	lines := strings.Split(msg, "\n")
	for _, l := range lines {
		fmt.Printf("    %v\n", l)
	}

}

func Log(c *git.Client, args []string) error {

	if len(args) != 0 {
		fmt.Fprintf(os.Stderr, "Usage: go-git log\nNo options are currently supported.\n")
		return errors.New("No options are currently supported for log")
	}

	/*
		repo, err := libgit.OpenRepository(c.GitDir.String())
		if err != nil {
			return err
		}
	*/
	head, err := c.GetHeadCommit()
	if err != nil {
		return err
	}

	ancestors, err := head.Ancestors(c)
	if err != nil {
		return err
	}

	dateCache = make(map[git.CommitID]time.Time)
	// GetDate is relatively expensive because it involves round trips to
	// the filesystem, so keep a cache to make the sort cheaper.
	sort.Slice(ancestors, func(i, j int) bool {
		var iDate, jDate time.Time
		var ok bool
		if iDate, ok = dateCache[ancestors[i]]; !ok {
			var err error
			iDate, err = ancestors[i].GetDate(c)
			if err != nil {
				panic(err)
			}
			dateCache[ancestors[i]] = iDate
		}
		if jDate, ok = dateCache[ancestors[j]]; !ok {
			var err error
			jDate, err = ancestors[j].GetDate(c)
			if err != nil {
				panic(err)
			}
			dateCache[ancestors[j]] = jDate
		}
		return jDate.Before(iDate)
	})

	for _, cmt := range ancestors {
		printCommit(c, cmt)
	}
	/*
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
	*/
	return nil

}
