package cmd

import (
	"io/ioutil"
	"log"
	"strings"

	"github.com/driusan/dgit/git"
)

func parseCommitFile(filename string) (string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	var strippedLines []string
	for _, line := range lines {
		if len(line) >= 1 && line[0] == '#' {
			continue
		}
		strippedLines = append(strippedLines, line)
	}
	return strings.Join(strippedLines, "\n"), nil
}

// Commit implements the command "git commit" in the repository pointed
// to by c.
func Commit(c *git.Client, args []string) (string, error) {
	// extract the message parameters that get passed directly
	//to commit-tree
	var messages []string
	var msgIncluded bool
	var opts git.CommitOptions
	for idx, val := range args {
		switch val {
		case "-m", "--message":
			msgIncluded = true
			messages = append(messages, args[idx+1])
		case "-F", "--file":
			msgIncluded = true
			msgFile, err := parseCommitFile(args[idx+1])
			if err != nil {
				return "", err
			}
			messages = append(messages, msgFile)
		case "--allow-empty-message":
			opts.AllowEmptyMessage = true
		case "-a", "--all":
			opts.All = true
		}
	}
	if !msgIncluded {
		s, err := git.StatusLong(
			c,
			nil,
			git.StatusUntrackedAll,
			"# ",
		)
		if err != nil {
			return "", err
		}

		c.GitDir.WriteFile("COMMIT_EDITMSG", []byte("\n"+s), 0660)
		if err := c.ExecEditor(c.GitDir.File("COMMIT_EDITMSG")); err != nil {
			log.Println(err)
		}
		msg, err := parseCommitFile(c.GitDir.String() + "/COMMIT_EDITMSG")
		if err != nil {
			return "", err
		}

		messages = append(messages, msg)
	}
	messageString := strings.Join(messages, "\n")
	cmt, err := git.Commit(c, opts, strings.TrimSpace(messageString), nil)
	if err != nil {
		return "", err
	}
	return cmt.String(), nil
}
