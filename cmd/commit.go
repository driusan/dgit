package cmd

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"../git"
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

	flags := flag.NewFlagSet("commit", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	var message []string
	flags.Var(newMultiStringValue(&message), "message", "Use the given message as the commit message")
	flags.Var(newMultiStringValue(&message), "m", "Alias for --message")

	var messageFile string
	flags.Var(newAliasedStringValue(&messageFile, ""), "file", "Take the commit message from the given file.")
	flags.Var(newAliasedStringValue(&messageFile, ""), "f", "Alias for --file")

	flags.BoolVar(&opts.Amend, "amend", false, "")
	flags.BoolVar(&opts.ResetAuthor, "reset-author", false, "")
	flags.BoolVar(&opts.AllowEmpty, "allow-empty", false, "")
	flags.BoolVar(&opts.AllowEmptyMessage, "allow-empty-message", false, "")

	edit := false
	flags.BoolVar(&edit, "edit", false, "")
	flags.BoolVar(&edit, "e", false, "Alias for --edit")

	_ = flags.Bool("no-edit", false, "")

	flags.StringVar(&opts.CleanupMode, "cleanup", "", "")

	flags.BoolVar(&opts.All, "all", false, "")
	flags.BoolVar(&opts.All, "a", false, "Alias for --all")

	flags.Parse(args)

	opts.NoEdit = true

	if messageFile != "" {
		f, err := ioutil.ReadFile(messageFile)
		if err != nil {
			return "", err
		}
		message = append(message, string(f))
	}

	if len(message) == 0 || edit {
		opts.NoEdit = false
	}

	finalMessage := strings.Join(message, "\n\n") + "\n"

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

		c.GitDir.WriteFile("COMMIT_EDITMSG", []byte(finalMessage+s), 0660)
		if err := c.ExecEditor(c.GitDir.File("COMMIT_EDITMSG")); err != nil {
			return "", err
		}
		m2, err := ioutil.ReadFile(c.GitDir.File("COMMIT_EDITMSG").String())
		if err != nil {
			return "", err
		}
		finalMessage = string(m2)
	}

	cmt, err := git.Commit(c, opts, git.CommitMessage(finalMessage), nil)
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
