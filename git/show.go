package git

import (
	"fmt"
	"strings"
)

type ShowOptions struct {
	Pretty string
}

// Show implementes the "git show" command.
func Show(c *Client, opts ShowOptions, objects []string) error {
	if len(objects) < 1 {
		return fmt.Errorf("Provide at least one commit.")
	}

	if opts.Pretty != "" && opts.Pretty != "raw" {
		return fmt.Errorf("Only raw format is supported, not %v", opts.Pretty)
	}

	commitIds := []CommitID{}

	for _, object := range objects {
		// Commits only for now
		commit, err := RevParseCommit(c, &RevParseOptions{}, object)
		if err != nil {
			return err
		}

		commitIds = append(commitIds, commit)
	}

	for _, commit := range commitIds {
		if opts.Pretty == "raw" {
			if err := showCommitRaw(commit, c); err != nil {
				return err
			}
		} else {
			if err := showCommit(commit, c); err != nil {
				return err
			}
		}
	}

	return nil
}

func showCommit(cmt CommitID, c *Client) error {
	author, err := cmt.GetAuthor(c)
	if err != nil {
		return err
	}

	date, err := cmt.GetDate(c)
	if err != nil {
		return err
	}

	msg, err := cmt.GetCommitMessage(c)
	if err != nil {
		return err
	}

	// Headers
	fmt.Printf("commit %v\nAuthor: %s\nDate: %v\n\n", cmt, author, date.Format("Mon Jan 2 15:04:05 2006 -0700"))

	// Commit message body
	lines := strings.Split(strings.TrimSpace(msg.String()), "\n")
	for _, l := range lines {
		fmt.Printf("    %v\n", l)
	}
	fmt.Printf("\n")

	return nil
}

func showCommitRaw(cmt CommitID, c *Client) error {
	author, err := cmt.GetAuthor(c)
	if err != nil {
		return err
	}

	date, err := cmt.GetDate(c)
	if err != nil {
		return err
	}

	msg, err := cmt.GetCommitMessage(c)
	if err != nil {
		return err
	}

	parents, err := cmt.Parents(c)
	if err != nil {
		return err
	}

	tree, err := cmt.TreeID(c)
	if err != nil {
		return err
	}

	// Headers
	fmt.Printf("commit %v\ntree %v\nparent %v\nauthor %s %v +0000\ncommiter %s %v +0000\n\n", cmt, tree, parents[0], author, date.Unix(), author, date.Unix())

	// Commit message body
	lines := strings.Split(strings.TrimSpace(msg.String()), "\n")
	for _, l := range lines {
		fmt.Printf("    %v\n", l)
	}
	fmt.Printf("\n")

	return nil
}
