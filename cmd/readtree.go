package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func ReadTree(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("read-tree", flag.ExitOnError)
	options := git.ReadTreeOptions{}
	flags.BoolVar(&options.Merge, "m", false, "Perform a merge. Will not run if you have unmerged entries")
	flags.BoolVar(&options.Reset, "reset", false, "Perform a merge. Will discard unmerged entries")
	flags.BoolVar(&options.Update, "u", false, "Update files in the work tree with the result of the merge")
	flags.BoolVar(&options.IgnoreWorktreeCheck, "i", false, "Disable work tree check")

	dryrun := flags.Bool("dry-run", false, "Do not update the index or files")
	n := flags.Bool("n", false, "Alias of --dry-run")

	flags.BoolVar(&options.TrivialMerge, "trivial", false, "Only perform three-way merge if there is no file level merging")
	flags.BoolVar(&options.AggressiveMerge, "aggressive", false, "Perform more aggressive trivial merges.")

	flags.StringVar(&options.Prefix, "prefix", "", "Use the index from prefix/. Must end in slash.")
	flags.StringVar(&options.ExcludePerDirectory, "exclude-per-directory", "", "Allow overwriting .gitignored files")

	flags.StringVar(&options.IndexOutput, "index-output", "index", "Name of the file to read the tree into")
	flags.BoolVar(&options.NoSparseCheckout, "no-sparse-checkout", false, "Disable sparse checkout")
	flags.BoolVar(&options.Verbose, "v", false, "Be verbose about updatig files.")

	flags.BoolVar(&options.Empty, "empty", false, "Instead of reading the treeish into the index, empty it")

	flags.Parse(args)
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(os.Stderr, "\nRead-Tree options:\n\n")
		flags.PrintDefaults()
	}
	args = flags.Args()
	options.DryRun = *dryrun || *n
	switch len(args) {
	case 0:
		if !options.Empty {
			flags.Usage()
			return fmt.Errorf("Invalid usage")
		}
		_, err := git.ReadTree(c, options, nil)
		if err != nil {
			return err
		}
	case 1:
		treeish, err := git.RevParseTreeish(c, &git.RevParseOptions{}, args[0])
		if err != nil {
			return err
		}
		_, err = git.ReadTree(c, options, treeish)
		if err != nil {
			return err
		}
	case 2:
		parent, err := git.RevParseTreeish(c, &git.RevParseOptions{}, args[0])
		if err != nil {
			return err
		}
		dst, err := git.RevParseTreeish(c, &git.RevParseOptions{}, args[1])
		if err != nil {
			return err
		}
		_, err = git.ReadTreeFastForward(c, options, parent, dst)
		if err != nil {
			return err
		}
	case 3:
		stage1, err := git.RevParseTreeish(c, &git.RevParseOptions{}, args[0])
		if err != nil {
			return err
		}
		stage2, err := git.RevParseTreeish(c, &git.RevParseOptions{}, args[1])
		if err != nil {
			return err
		}
		stage3, err := git.RevParseTreeish(c, &git.RevParseOptions{}, args[2])
		if err != nil {
			return err
		}
		_, err = git.ReadTreeMerge(c, options, stage1, stage2, stage3)
		if err != nil {
			return err
		}
	default:
		flags.Usage()
		return fmt.Errorf("Invalid usage")
	}
	return nil
}
