package git

import (
	"fmt"
	"strings"
)

type FormatString string

func (f FormatString) FormatCommit(c *Client, cmt CommitID) (string, error) {
	if f == "" || f == "medium" {
		output, err := formatCommitMedium(cmt, c)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%v\n", output), nil
	}
	if f == "raw" {
		output := fmt.Sprintf("commit %v\n", cmt)
		cmtObject, err := c.GetCommitObject(cmt)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%v%v\n", output, cmtObject), nil
	}

	return "", fmt.Errorf("Format %s is not supported.\n", f)
}

type ShowOptions struct {
	DiffOptions
	Format FormatString
}

// Show implementes the "git show" command.
func Show(c *Client, opts ShowOptions, objects []string) error {
	if len(objects) < 1 {
		return fmt.Errorf("Provide at least one commit.")
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
		output, err := opts.Format.FormatCommit(c, commit)
		if err != nil {
			return err
		}
		fmt.Printf("%v", output)
	}

	return nil
}

func formatCommitMedium(cmt CommitID, c *Client) (string, error) {
	author, err := cmt.GetAuthor(c)
	if err != nil {
		return "", err
	}

	date, err := cmt.GetDate(c)
	if err != nil {
		return "", err
	}

	msg, err := cmt.GetCommitMessage(c)
	if err != nil {
		return "", err
	}

	// Headers
	output := fmt.Sprintf("commit %v\nAuthor: %s\nDate: %v\n\n", cmt, author, date.Format("Mon Jan 2 15:04:05 2006 -0700"))

	// Commit message body
	lines := strings.Split(strings.TrimSpace(msg.String()), "\n")
	for _, l := range lines {
		output = fmt.Sprintf("%v    %v\n", output, l)
	}

	return output, nil
}
