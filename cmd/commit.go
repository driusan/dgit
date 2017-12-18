package cmd

import (
	"fmt"
	"log"

	"github.com/driusan/dgit/git"
)

// Commit implements the command "git commit" in the repository pointed
// to by c.
func Commit(c *git.Client, args []string) (string, error) {
	// get the parent commit, if it exists
	var commitTreeArgs []string
	if parentCommit, err := c.GetHeadCommit(); err == nil {
		commitTreeArgs = []string{"-p", parentCommit.String()}
	}

	// extract the message parameters that get passed directly
	//to commit-tree
	var messages []string
	var msgIncluded bool
	for idx, val := range args {
		switch val {
		case "-m", "-F":
			msgIncluded = true
			messages = append(messages, args[idx:idx+2]...)
		}
	}
	if !msgIncluded {
		s, err := getStatus(c, "# ")
		if err != nil {
			return "", err
		}

		c.GitDir.WriteFile("COMMIT_EDITMSG", []byte("\n"+s), 0660)
		if err := c.ExecEditor(c.GitDir.File("COMMIT_EDITMSG")); err != nil {
			log.Println(err)
		}
		commitTreeArgs = append(commitTreeArgs, "-F", c.GitDir.File("COMMIT_EDITMSG").String())
	}
	commitTreeArgs = append(commitTreeArgs, messages...)

	// write the current index tree and get the SHA1
	treeSha1 := WriteTree(c, nil)
	commitTreeArgs = append(commitTreeArgs, treeSha1)

	// write the commit tree
	commitSha1, err := CommitTree(c, commitTreeArgs)
	if err != nil {
		return "", err
	}
	file := c.GitDir.File("COMMIT_EDITMSG")
	msg, _ := file.ReadFirstLine()
	if msg == "" {
		msg = "commit from go-git"
	}
	refmsg := fmt.Sprintf("commit: %s (go-git)", msg)

	oldHead, err := c.GetHeadCommit()
	if err != nil {
		return "", err
	}
	err = git.UpdateRef(c, git.UpdateRefOptions{OldValue: oldHead, CreateReflog: true}, "HEAD", commitSha1, refmsg)
	return commitSha1.String(), err
}
