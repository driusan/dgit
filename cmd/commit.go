package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/driusan/dgit/git"
)

func printNoUserMessage(committer git.Person) {
	fmt.Fprintf(os.Stderr, ` Committer: %v
Your name and email address were configured automatically based
on your username and hostname. Please check that they are accurate.
You can suppress this message by setting them explicitly. Run the
following commands:

	dgit config --global user.name 'Your Name'
	dgit config --global user.email your@email

After doing this, you may fix the identity userd for this commit with:

	dgit commit --amend --reset-author
`, committer)
}

// Commit implements the command "git commit" in the repository pointed
// to by c.
func Commit(c *git.Client, args []string) (string, error) {
	// extract the message parameters that get passed directly
	//to commit-tree
	var opts git.CommitOptions
	var message string
	for idx, val := range args {
		switch val {
		case "-m", "--message":
			if message == "" {
				message = args[idx+1] + "\n"
			} else {
				message = "\n" + args[idx+1] + "\n"
			}
			opts.NoEdit = true
		case "-F", "--file":
			f, err := ioutil.ReadFile(args[idx+1])
			if err != nil {
				return "", err
			}
			if message == "" {
				message = string(f)
			} else {
				message = "\n" + string(f) + "\n"
			}
			opts.NoEdit = true
		case "--amend":
			opts.Amend = true
		case "--reset-author":
			opts.ResetAuthor = true
		case "--allow-empty":
			opts.AllowEmpty = true
		case "--allow-empty-message":
			opts.AllowEmptyMessage = true
		case "--edit", "-e":
			opts.NoEdit = false
		case "--no-edit":
			opts.NoEdit = true
		case "--cleanup":
			opts.CleanupMode = args[idx+1]
		case "-a", "--all":
			opts.All = true
		}
	}
	if !opts.NoEdit {
		s, err := git.StatusLong(
			c,
			nil,
			git.StatusUntrackedAll,
			"# ",
		)
		if err != nil {
			return "", err
		}

		c.GitDir.WriteFile("COMMIT_EDITMSG", []byte(message+"\n"+s), 0660)
		if err := c.ExecEditor(c.GitDir.File("COMMIT_EDITMSG")); err != nil {
			log.Println(err)
		}
		m2, err := ioutil.ReadFile(c.GitDir.File("COMMIT_EDITMSG").String())
		if err != nil {
			return "", err
		}
		message = string(m2)
	}
	cmt, err := git.Commit(c, opts, git.CommitMessage(message), nil)
	switch err {
	case git.NoGlobalConfig:
		committer, _ := c.GetCommitter(nil)
		printNoUserMessage(committer)
		fallthrough
	case nil:
		return cmt.String(), nil
	default:
		return "", err
	}
}
