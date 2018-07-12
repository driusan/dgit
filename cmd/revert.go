package cmd

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
        "os"

	"github.com/driusan/dgit/git"
)

func Revert(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("revert", flag.ExitOnError)
        flags.SetOutput(os.Stdout)
	flags.Usage = func() {
		flag.Usage()
                fmt.Printf("\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.RevertOptions{}

	flags.BoolVar(&opts.Edit, "edit", true, "Allow the commit message to be edited prior to committing")
	e := flags.Bool("e", false, "Alias of --edit")
	noedit := flags.Bool("no-edit", false, "Do not open an editor automatically (negate --edit)")

	flags.IntVar(&opts.MergeParent, "mainline", 0, "Choose which parent of a merge commit to cherry-pick (1 indexed)")
	m := flags.Int("m", 0, "Alias of --mainline")

	flags.BoolVar(&opts.NoCommit, "no-commit", false, "Do not create a commit, apply the change against your index instead")
	n := flags.Bool("n", false, "Alias of --no-commit")

	flags.BoolVar(&opts.SignOff, "signoff", false, "Add a Signed-off-by line at the end of the commit message")
	s := flags.Bool("s", false, "Alias of --signoff")

	flags.StringVar(&opts.MergeStrategy, "strategy", "", "Use the given merge strategy")

	flags.StringVar(&opts.MergeStrategyOption, "strategy-option", "", "Pass a merge strategy specific option to the merge strategy")
	X := flags.String("X", "", "Alias of --strategy-option")

	// FIXME: Add --gpg-sign=<keyid>

	// Sequencer subcommands
	flags.BoolVar(&opts.Continue, "continue", false, "Continue the operation in progress using the information in .git/sequencer")
	flags.BoolVar(&opts.Quit, "quit", false, "Forget the current operation in progress.")
	flags.BoolVar(&opts.Abort, "abort", false, "Cancel the operation and return to the pre-sequence state")

	flags.Parse(args)

	if *e {
		opts.Edit = true
	}
	if *noedit {
		opts.Edit = false
	}
	if *m > 0 {
		opts.MergeParent = *m
	}
	if *n {
		opts.NoCommit = true
	}

	if *s {
		opts.SignOff = true
	}
	if *X != "" {
		opts.MergeStrategyOption = *X
	}
	cstrings := flags.Args()
	commits := make([]git.Commitish, len(cstrings), len(cstrings))
	for i := range cstrings {
		com, err := git.RevParseCommitish(c, &git.RevParseOptions{}, cstrings[i])
		if err != nil {
			return err
		}
		commits[i] = com

	}
	if len(commits) < 0 {
		return fmt.Errorf("No commit provided to revert")
	}
	cid, err := commits[0].CommitID(c)
	if err != nil {
		return err
	}

	cmessage, err := cid.GetCommitMessage(c)
	if err != nil {
		return err
	}

	editmessage := fmt.Sprintf("Revert \"%v\"\n\nThis reverts commit %v\n", cmessage.Subject(), cid.String())

	st, err := git.StatusLong(c, nil, git.StatusUntrackedAll, "# ")
	if err != nil {
		return err
	}
	if err := c.GitDir.WriteFile("COMMIT_EDITMSG", []byte(editmessage+"\n"+st), 0660); err != nil {
		return err
	}
	if err := c.ExecEditor(c.GitDir.File("COMMIT_EDITMSG")); err != nil {
		log.Println(err)
	}
	message, err := ioutil.ReadFile(c.GitDir.File("COMMIT_EDITMSG").String())
	if err != nil {
		return err
	}
	if err := git.Revert(c, opts, commits); err != nil {
		return err
	}
	if !opts.NoCommit {
		if _, err := git.Commit(c, git.CommitOptions{
			NoEdit:  !opts.Edit,
			Signoff: opts.SignOff,
			All:     true,
		},
			git.CommitMessage(message),
			nil,
		); err != nil {
			return err
		}
	}
	return nil
}
