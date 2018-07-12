package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

// Implements the git checkout command line parsing.
func Checkout(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("checkout", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	options := git.CheckoutOptions{}

	quiet := flags.Bool("quiet", false, "Quiet. Suppress feedback messages.")
	q := flags.Bool("q", false, "Alias of --quiet.")

	progress := flags.Bool("progress", true, "Report progress to standard error stream")
	noprogress := flags.Bool("no-progress", false, "Override --progress and suppress progress reporting")

	force := flags.Bool("force", false, "When switching branches, proceed even if the index differs from HEAD")
	f := flags.Bool("f", false, "Alias of --force.")

	ours := flags.Bool("ours", false, "Use stage2 for checking out unmerged paths from the index")
	theirs := flags.Bool("theirs", false, "Use stage3 for checking out unmerged paths from the index")

	b := flags.String("b", "", "Create a new branch")
	B := flags.String("B", "", "Create a new branch, overwriting if it exists")

	track := flags.String("track", "", "When creating a new branch, set upstream to branch")
	t := flags.String("t", "", "Alias of --track")
	notrack := flags.Bool("no-track", false, "Override --track and do not set upstream")

	flags.BoolVar(&options.CreateReflog, "l", true, "Create the new branch's reflog")
	flags.BoolVar(&options.Detach, "detach", false, "Checkout in detached head state.")
	orphan := flags.String("orphan", "", "Create a new branch with no parents")

	flags.BoolVar(&options.IgnoreSkipWorktreeBits, "ignore-skip-worktree-bits", false, "Unused (spare checkout not supported)")

	merge := flags.Bool("merge", false, "Perform three-way merge with local modifications if switching branches")
	m := flags.Bool("m", false, "Alias of --merge")

	flags.StringVar(&options.ConflictStyle, "conflict", "merge", "Use style to display conflicts (valid values are merge or diff3) (Not implemented)")

	patch := flags.Bool("patch", false, "Interactively select hunks to discard (not implemented")
	p := flags.Bool("p", false, "Alias of --patch")

	flags.BoolVar(&options.IgnoreOtherWorktrees, "ignore-other-worktrees", false, "Unused, for compatibility with git only.")

	flags.Parse(args)
	files := flags.Args()

	options.Quiet = *quiet || *q
	options.Progress = *progress && !*noprogress
	options.Force = *force || *f
	if *ours && *theirs {
		return fmt.Errorf("--ours and --theirs are mutually exclusive.")
	} else if *ours {
		options.Stage = git.Stage2
	} else if *theirs {
		options.Stage = git.Stage3
	}

	if *b != "" && *B != "" {
		return fmt.Errorf("-b and -B are mutually exclusive.")
	} else if *b != "" {
		options.Branch = *b
	} else if *B != "" {
		options.Branch = *B
		options.ForceBranch = true
	}

	if *notrack && (*track != "" || *t != "") {
		return fmt.Errorf("--track and --no-track are mutually exclusive.")
	} else if !*notrack {
		if *track != "" && *t != "" {
			return fmt.Errorf("--track and -t are mutually exclusive.")
		} else if *track != "" {
			options.Track = *track
		} else if *t != "" {
			options.Track = *t
		}
	}

	if *orphan != "" {
		if options.Branch != "" {
			return fmt.Errorf("--orphan is incompatible with -b/-B")
		}
		options.Branch = *orphan
		options.OrphanBranch = true
	}

	options.Merge = *merge || *m
	options.Patch = *patch || *p

	var thing string = "HEAD"
	if len(files) > 0 {
		f := git.File(files[0])
		if !f.Exists() {
			thing = files[0]
			files = files[1:]
		}
	}

	// Convert from string to git.File
	gfiles := make([]git.File, len(files))
	for i, f := range files {
		gfiles[i] = git.File(f)
	}

	return git.Checkout(c, options, thing, gfiles)
}
