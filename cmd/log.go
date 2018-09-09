package cmd

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

// Since libgit is somewhat out of our control and we can't implement
// a fmt.Stringer interface there, we use this helper.
func printCommit(c *git.Client, cmt git.CommitID) error {
	author, err := cmt.GetAuthor(c)
	if err != nil {
		return err
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
		return err
	}
	fmt.Printf("Author: %v\nDate:   %v\n\n", author, date.Format("Mon Jan 2 15:04:05 2006 -0700"))
	log.Printf("Commit %v\n", cmt)

	msg, err := cmt.GetCommitMessage(c)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(msg.String()), "\n")
	for _, l := range lines {
		fmt.Printf("    %v\n", l)
	}
	fmt.Printf("\n")
	return nil
}

func Log(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("log", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	flags.Var(newNotimplBoolValue(), "follow", "Not implemented")
	flags.Var(newNotimplBoolValue(), "no-decorate", "Not implemented")
	flags.Var(newNotimplStringValue(), "decorate", "Not implemented")
	flags.Var(newNotimplStringValue(), "decorate-refs", "Not implemented")
	flags.Var(newNotimplStringValue(), "decorate-refs-exclude", "Not implemented")
	flags.Var(newNotimplBoolValue(), "source", "Not implemented")
	flags.Var(newNotimplBoolValue(), "use-mailmap", "Not implemented")
	flags.Var(newNotimplBoolValue(), "full-diff", "Not implemented")
	flags.Var(newNotimplStringValue(), "log-size", "Not implemented")
	flags.Var(newNotimplStringValue(), "L", "Not implemented")

	flags.Parse(args)

	if flags.NArg() > 1 {
		fmt.Fprintf(flag.CommandLine.Output(), "Paths are not yet implemented, just the revision")
		flags.Usage()
		os.Exit(2)
	}

	var commit git.Commitish
	var err error
	if flags.NArg() == 0 {
		commit, err = git.RevParseCommitish(c, &git.RevParseOptions{}, "HEAD")
	} else {
		commit, err = git.RevParseCommitish(c, &git.RevParseOptions{}, flags.Arg(0))
	}
	if err != nil {
		return err
	}
	return git.RevListCallback(c, git.RevListOptions{Quiet: true}, []git.Commitish{commit}, nil, func(s git.Sha1) error {
		return printCommit(c, git.CommitID(s))
	})

}
